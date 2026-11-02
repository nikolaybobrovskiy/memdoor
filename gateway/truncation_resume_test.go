package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

func resumeFixture(cut llm.MessageParam) []llm.MessageParam {
	return []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("build the frontend")),
		{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{llm.NewToolUseBlock("t0", `{"cmd":"ls"}`, "bash")}},
		llm.NewUserMessage(llm.NewToolResultBlock("t0", "README.md", false)),
		cut,
	}
}

func roles(conv []llm.MessageParam) string {
	var out []string
	for _, m := range conv {
		out = append(out, string(m.Role))
	}
	return strings.Join(out, ",")
}

// Every message must carry content: an empty one is what the provider refused
// (HTTP 400, 2026-11-02), and every later turn of that conversation with it.
func assertSendable(t *testing.T, conv []llm.MessageParam) {
	t.Helper()
	asked := map[string]bool{}
	for i, m := range conv {
		if len(m.Content) == 0 {
			t.Fatalf("message %d (%s) has no content: %s", i, m.Role, roles(conv))
		}
		if i > 0 && m.Role == conv[i-1].Role {
			t.Fatalf("two %s messages in a row at %d: %s", m.Role, i, roles(conv))
		}
		for _, b := range m.Content {
			if b.OfToolUse != nil {
				asked[b.OfToolUse.ID] = true
			}
			if b.OfToolResult != nil {
				if !asked[b.OfToolResult.ToolUseID] {
					t.Fatalf("result for unknown call %s", b.OfToolResult.ToolUseID)
				}
				delete(asked, b.OfToolResult.ToolUseID)
			}
		}
	}
	if len(asked) > 0 {
		t.Fatalf("calls with no result: %v", asked)
	}
}

func lastText(m llm.MessageParam) string {
	b := m.Content[len(m.Content)-1]
	if b.OfText == nil {
		return ""
	}
	return b.OfText.Text
}

// The live failure: a reply cut off inside its one tool call reaches the turn
// with no content. Resuming must drop it and say why, not send it back empty.
func TestResumeDropsAReplyCutOffWithNothingInIt(t *testing.T) {
	conv := resumeFixture(llm.MessageParam{Role: llm.MessageParamRoleAssistant})
	before, _ := json.Marshal(conv)

	got := resumeConversation(conv, nil, resumePrompt)

	assertSendable(t, got)
	if roles(got) != "user,assistant,user" {
		t.Fatalf("the empty reply is dropped, the note joins the last user message: %s", roles(got))
	}
	last := got[len(got)-1]
	if last.Content[0].OfToolResult == nil || lastText(last) != emptyCutPrompt {
		t.Fatalf("tool result kept, then the empty-cut note: %+v", last.Content)
	}
	if after, _ := json.Marshal(conv); string(after) != string(before) {
		t.Fatal("the caller's conversation was modified")
	}
}

// Whitespace is not something to resume from either.
func TestResumeTreatsABlankReplyAsEmpty(t *testing.T) {
	got := resumeConversation(resumeFixture(llm.MessageParam{Role: llm.MessageParamRoleAssistant,
		Content: []llm.ContentBlockParamUnion{llm.NewTextBlock("  \n")}}), nil, resumePrompt)
	assertSendable(t, got)
	if lastText(got[len(got)-1]) != emptyCutPrompt {
		t.Fatalf("a blank reply resumes like an empty one: %s", roles(got))
	}
}

// A reply with text keeps it, and the resume instruction follows as the user.
func TestResumeKeepsTheTextOfACutOffReply(t *testing.T) {
	got := resumeConversation(resumeFixture(llm.MessageParam{Role: llm.MessageParamRoleAssistant,
		Content: []llm.ContentBlockParamUnion{llm.NewTextBlock("Scaffolding the Vue app: first package.json, then")}}), nil, resumePrompt)
	assertSendable(t, got)
	if roles(got) != "user,assistant,user,assistant,user" {
		t.Fatalf("text kept, then the instruction: %s", roles(got))
	}
	if !strings.HasPrefix(lastText(got[3]), "Scaffolding") || lastText(got[4]) != resumePrompt {
		t.Fatalf("got %+v", got[3:])
	}
}

// Tool calls of the cut reply that already ran keep their call AND result;
// a call with no result (the unfinished one) is dropped rather than orphaned.
func TestResumeKeepsAnsweredCallsAndDropsTheUnfinishedOne(t *testing.T) {
	cut := llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{
		llm.NewTextBlock("Writing both files."),
		llm.NewToolUseBlock("t1", `{"path":"a.ts"}`, "apply_patch"),
		llm.NewToolUseBlock("t2", `{"path":"b.ts"`, "apply_patch"),
	}}
	results := []llm.ContentBlockParamUnion{llm.NewToolResultBlock("t1", "ok", false)}

	got := resumeConversation(resumeFixture(cut), results, resumePrompt)

	assertSendable(t, got)
	reply, user := got[3], got[4]
	if len(reply.Content) != 2 || reply.Content[1].OfToolUse == nil || reply.Content[1].OfToolUse.ID != "t1" {
		t.Fatalf("text and the answered call: %+v", reply.Content)
	}
	if user.Content[0].OfToolResult == nil || user.Content[0].OfToolResult.ToolUseID != "t1" || lastText(user) != resumePrompt {
		t.Fatalf("its result, then the instruction: %+v", user.Content)
	}
}

// Giving up keeps a conversation the next turn can still send.
func TestGivingUpLeavesASendableConversation(t *testing.T) {
	empty := settleCutReply(resumeFixture(llm.MessageParam{Role: llm.MessageParamRoleAssistant}), nil, "")
	assertSendable(t, empty)
	if roles(empty) != "user,assistant,user" {
		t.Fatalf("the empty reply is dropped and nothing added: %s", roles(empty))
	}
	if len(empty[2].Content) != 1 {
		t.Fatalf("no instruction when giving up: %+v", empty[2].Content)
	}

	cut := llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{
		llm.NewTextBlock("Half an answer"), llm.NewToolUseBlock("t9", `{`, "apply_patch")}}
	text := settleCutReply(resumeFixture(cut), nil, "")
	assertSendable(t, text)
	if roles(text) != "user,assistant,user,assistant" || len(text[3].Content) != 1 {
		t.Fatalf("the text stays, the unanswered call goes: %s %+v", roles(text), text[3].Content)
	}
}

// A call cut mid-arguments may have run on its partial JSON and failed: its
// result does not make it replayable, and neither call nor result is kept.
func TestResumeDropsACallWithPartialArgumentsEvenWithAResult(t *testing.T) {
	cut := llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{
		llm.NewTextBlock("Writing the file."),
		llm.NewToolUseBlock("t3", `{"path":"big.vue","patch":"<templ`, "apply_patch"),
	}}
	results := []llm.ContentBlockParamUnion{llm.NewToolResultBlock("t3", "invalid arguments", true)}
	for name, got := range map[string][]llm.MessageParam{
		"resume":  resumeConversation(resumeFixture(cut), results, resumePrompt),
		"give up": settleCutReply(resumeFixture(cut), results, ""),
	} {
		assertSendable(t, got)
		if _, err := json.Marshal(got); err != nil {
			t.Fatalf("%s: the conversation must serialize: %v", name, err)
		}
		for _, m := range got {
			for _, b := range m.Content {
				if b.OfToolUse != nil && b.OfToolUse.ID == "t3" || b.OfToolResult != nil && b.OfToolResult.ToolUseID == "t3" {
					t.Fatalf("%s: the partial call or its result was kept: %s", name, roles(got))
				}
			}
		}
	}
}

// Escalating to the cap the reply already had replays the same generation:
// when the model's ceiling is the normal cap, recovery starts at resume.
func TestNoEscalationWhenTheCapIsAlreadyTheCeiling(t *testing.T) {
	r, err := newTurnRecovery(65536, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if step := r.Next(); step.Kind != RecoveryResume || step.Attempt != 1 {
		t.Fatalf("first step at the ceiling: %+v", step)
	}
	r, _ = newTurnRecovery(16384, 32768)
	if step := r.Next(); step.Kind != RecoveryEscalate || step.MaxTokens != 32768 {
		t.Fatalf("room to grow still escalates: %+v", step)
	}
}
