package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"memdoor/pkg/llm"
)

// RecoveryKind is what to do about a reply the model did not finish.
type RecoveryKind int

const (
	// RecoveryEscalate retries the SAME request at a larger output cap, on the
	// theory that the model simply needed more room. Once per turn.
	RecoveryEscalate RecoveryKind = iota
	// RecoveryResume keeps the truncated text and asks the model to pick up
	// mid-thought.
	RecoveryResume
	// RecoveryExhausted means stop and say so, rather than presenting a
	// half-written reply as an answer.
	RecoveryExhausted
)

// RecoveryStep is one decision from a TruncationRecovery.
type RecoveryStep struct {
	Kind RecoveryKind
	// MaxTokens is the escalated cap, set only for RecoveryEscalate.
	MaxTokens int64
	// Prompt is the resume instruction, set only for RecoveryResume.
	Prompt string
	// Attempt is which resume this is, 1-based, set only for RecoveryResume.
	Attempt int
}

// TruncationRecovery decides how a turn responds to a reply cut off at the
// output cap.
//
// It is an interface because the POLICY is the interesting part and the turn
// driver should not own it: the loop asks "what now?" and acts, while the
// sequence — escalate once, then resume a bounded number of times, then give up
// — is decided here and can be tested without running a single inference.
type TruncationRecovery interface {
	// Next returns what to do about the truncated reply just received, and
	// advances the policy. Calling it is what consumes an attempt.
	Next() RecoveryStep
}

// resumePrompt is the instruction handed back to a model that was cut off.
//
// Taken from Claude Code (query.ts:1223), whose wording is deliberate: no
// apology and no recap, because a model that re-explains itself spends the new
// budget on the same tokens it just lost — and "break remaining work into
// smaller pieces" is the only instruction that changes the outcome rather than
// repeating the attempt.
const resumePrompt = "Output token limit hit. Resume directly — no apology, no recap of " +
	"what you were doing. Pick up mid-thought if that is where the cut " +
	"happened. Break remaining work into smaller pieces."

// maxResumeAttempts bounds stage two. Claude Code uses 3
// (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT); a model that cannot finish in three
// resumes is not going to on the fourth, and each one is a full generation.
const maxResumeAttempts = 3

type truncationRecovery struct {
	escalatedCap int64
	escalated    bool
	resumes      int
}

// newTurnRecovery is the policy for a turn whose normal cap is normalCap:
// when the escalated cap is no larger (the model's own ceiling is already
// the normal cap), escalating would replay the same generation at the same
// limit, so recovery starts at the resume stage.
func newTurnRecovery(normalCap, escalatedCap int64) (TruncationRecovery, error) {
	r, err := NewTruncationRecovery(escalatedCap)
	if err != nil {
		return nil, err
	}
	if escalatedCap <= normalCap {
		r.(*truncationRecovery).escalated = true
	}
	return r, nil
}

// NewTruncationRecovery returns the policy for one turn.
//
// escalatedCap must exceed the turn's normal cap — retrying at the same limit
// would burn another full generation only to hit it again — so a bad value
// fails here rather than wasting a model call discovering it.
func NewTruncationRecovery(escalatedCap int64) (TruncationRecovery, error) {
	if escalatedCap <= 0 {
		return nil, fmt.Errorf("escalated cap must be positive, got %d", escalatedCap)
	}
	return &truncationRecovery{escalatedCap: escalatedCap}, nil
}

func (r *truncationRecovery) Next() RecoveryStep {
	if !r.escalated {
		r.escalated = true
		return RecoveryStep{Kind: RecoveryEscalate, MaxTokens: r.escalatedCap}
	}
	if r.resumes < maxResumeAttempts {
		r.resumes++
		return RecoveryStep{Kind: RecoveryResume, Prompt: resumePrompt, Attempt: r.resumes}
	}
	return RecoveryStep{Kind: RecoveryExhausted}
}

// emptyCutPrompt replaces resumePrompt when nothing of the cut-off reply
// survives: there is no thought to pick up, only a step that was too big.
const emptyCutPrompt = "Output token limit hit before any of your reply could be used " +
	"(most likely one tool call too large to finish), so it was discarded. Do not " +
	"retry it in one piece: break the work into smaller steps, for example write a " +
	"large file with several smaller apply_patch calls."

// resumeConversation rebuilds the end of conversation for a resume, where
// the last entry is the cut-off reply and toolResults answer the tool calls
// of it that already ran.
//
// Sending the cut-off reply back as it was is not a valid request. Its
// unfinished tool call is dropped by the provider, which can leave the reply
// with no content at all (refused upstream: HTTP 400, live 2026-11-02, and the
// conversation stayed broken for every later turn); and a tool call that ran
// but whose result is never sent is an orphan the next inference rejects.
//
// So only what can be carried forward is kept: the reply's text and the tool
// calls that have a result, followed by one user message with those results
// and the resume instruction. When nothing is left, the reply is dropped and
// the instruction says so (emptyCutPrompt), joined to the user message the
// conversation already ends with rather than making a second one in a row.
func resumeConversation(conversation []llm.MessageParam, toolResults []llm.ContentBlockParamUnion, prompt string) []llm.MessageParam {
	return settleCutReply(conversation, toolResults, prompt)
}

// settleCutReply is resumeConversation without an instruction (prompt ""):
// what a turn that gives up on a cut-off reply keeps, so the saved
// conversation stays one every later request can carry.
func settleCutReply(conversation []llm.MessageParam, toolResults []llm.ContentBlockParamUnion, prompt string) []llm.MessageParam {
	n := len(conversation)
	if n == 0 {
		if prompt == "" {
			return conversation
		}
		return []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(prompt))}
	}
	cut := conversation[n-1]
	if cut.Role != llm.MessageParamRoleAssistant {
		// No reply to replace: nothing is dropped.
		if prompt == "" {
			return conversation
		}
		return appendToLastUser(conversation[:n:n], llm.NewTextBlock(prompt))
	}
	out := conversation[: n-1 : n-1] // appends below never write into the caller's array

	answered := map[string]bool{}
	for _, r := range toolResults {
		if r.OfToolResult != nil {
			answered[r.OfToolResult.ToolUseID] = true
		}
	}
	var kept []llm.ContentBlockParamUnion
	keptCall := map[string]bool{}
	for _, b := range cut.Content {
		switch {
		case b.OfText != nil && strings.TrimSpace(b.OfText.Text) != "":
			kept = append(kept, b)
		// A call cut mid-arguments can still have run (and failed) on its
		// partial JSON; its result does not make it replayable.
		case b.OfToolUse != nil && answered[b.OfToolUse.ID] && json.Valid(b.OfToolUse.Input):
			kept = append(kept, b)
			keptCall[b.OfToolUse.ID] = true
		}
	}

	if len(kept) > 0 {
		out = append(out, llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: kept})
		var user []llm.ContentBlockParamUnion
		for _, r := range toolResults {
			if r.OfToolResult != nil && keptCall[r.OfToolResult.ToolUseID] {
				user = append(user, r)
			}
		}
		if prompt != "" {
			user = append(user, llm.NewTextBlock(prompt))
		}
		if len(user) > 0 {
			out = append(out, llm.NewUserMessage(user...))
		}
		return out
	}
	if prompt == "" {
		return out
	}
	return appendToLastUser(out, llm.NewTextBlock(emptyCutPrompt))
}

// appendToLastUser adds block to the user message conversation ends with,
// or as a new user message after an assistant one. The caller's messages are
// not modified.
func appendToLastUser(conversation []llm.MessageParam, block llm.ContentBlockParamUnion) []llm.MessageParam {
	n := len(conversation)
	if n > 0 && conversation[n-1].Role == llm.MessageParamRoleUser {
		last := conversation[n-1]
		content := append(append([]llm.ContentBlockParamUnion{}, last.Content...), block)
		return append(conversation[:n-1:n-1], llm.MessageParam{Role: last.Role, Content: content})
	}
	return append(conversation[:n:n], llm.NewUserMessage(block))
}
