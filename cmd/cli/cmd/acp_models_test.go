package cmd

import (
	"context"
	"testing"

	"github.com/coder/acp-go-sdk"
)

func TestTheModelChoicesComeFromTheEnvironment(t *testing.T) {
	t.Setenv("MEMDOOR_ACP_MODELS", "bridge:claude-haiku-5-5|Claude Haiku 5.5; openrouter-free:cohere/north-mini-code:free|North Mini\n default|x ;;opencode:qwen3.8-flash")
	t.Setenv("MEMDOOR_ACP_DEFAULT_NAME", "Haiku")
	models, def := acpModelsFromEnv()
	want := []acpModel{{"bridge:claude-haiku-5-5", "Claude Haiku 5.5"}, {"openrouter-free:cohere/north-mini-code:free", "North Mini"}, {"opencode:qwen3.8-flash", "opencode:qwen3.8-flash"}}
	if len(models) != len(want) || def != "Haiku" {
		t.Fatalf("%+v %q", models, def)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("%d: %+v", i, models[i])
		}
	}
	t.Setenv("MEMDOOR_ACP_MODELS", "")
	if m, _ := acpModelsFromEnv(); len(m) != 0 {
		t.Fatalf("none configured: %+v", m)
	}
}

func modelSelect(t *testing.T, opts []acp.SessionConfigOption) *acp.SessionConfigOptionSelect {
	t.Helper()
	if len(opts) != 1 || opts[0].Select == nil || opts[0].Select.Id != acpModelOption {
		t.Fatalf("model option: %+v", opts)
	}
	return opts[0].Select
}

// A session offers the model option; picking a model pins it on that
// session's conversation, "default" releases it, an unknown value is refused.
func TestAnEditorPicksTheSessionsModel(t *testing.T) {
	runner := &fakeRunner{}
	agent := newACPAgent(runner)
	agent.models = []acpModel{{"bridge:claude-haiku-5-5", "Claude Haiku 5.5"}, {"opencode:qwen3.8-flash", "Qwen3.8 Flash"}}
	agent.defaultModelName = "DeepSeek V4.1 Flash"
	ctx := context.Background()

	sess, err := agent.NewSession(ctx, acp.NewSessionRequest{Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	sel := modelSelect(t, sess.ConfigOptions)
	if sel.CurrentValue != acpModelDefault || sel.Category == nil || *sel.Category != acp.SessionConfigOptionCategoryModel {
		t.Fatalf("starts on the default: %+v", sel)
	}
	opts := *sel.Options.Ungrouped
	if len(opts) != 3 || opts[0].Name != "Default (DeepSeek V4.1 Flash)" || opts[2].Value != "opencode:qwen3.8-flash" {
		t.Fatalf("choices: %+v", opts)
	}

	set := func(v string) (acp.SetSessionConfigOptionResponse, error) {
		return agent.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{
			SessionId: sess.SessionId, ConfigId: acpModelOption, Value: acp.SessionConfigValueId(v)}})
	}
	resp, err := set("opencode:qwen3.8-flash")
	if err != nil {
		t.Fatal(err)
	}
	if modelSelect(t, resp.ConfigOptions).CurrentValue != "opencode:qwen3.8-flash" {
		t.Fatalf("the response carries the new value: %+v", resp)
	}
	if _, err := set("default"); err != nil {
		t.Fatal(err)
	}
	if _, err := set("nobody:model"); err == nil {
		t.Fatal("a model not offered is refused")
	}
	runner.mu.Lock()
	got := append([]string{}, runner.models...)
	runner.mu.Unlock()
	if len(got) != 2 || got[0] != "conv-1=opencode:qwen3.8-flash" || got[1] != "conv-1=" {
		t.Fatalf("pins sent: %v", got)
	}
}

// Without configured choices the session offers no option and refuses one.
func TestNoChoicesNoModelOption(t *testing.T) {
	agent := newACPAgent(&fakeRunner{})
	sess, err := agent.NewSession(context.Background(), acp.NewSessionRequest{Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.ConfigOptions) != 0 {
		t.Fatalf("%+v", sess.ConfigOptions)
	}
	if _, err := agent.SetSessionConfigOption(context.Background(), acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{
		SessionId: sess.SessionId, ConfigId: acpModelOption, Value: "x"}}); err == nil {
		t.Fatal("refused")
	}
}
