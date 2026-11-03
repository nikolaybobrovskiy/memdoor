package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/coder/acp-go-sdk"
)

// THE EDITOR PICKS THE MODEL (2026-11-03, JetBrains Rider and Hermes over ACP).
// The gateway already pins a model per conversation (/api/route, any
// connected provider's model, "provider:model"), the window's /model; an
// editor had no way to reach it. The session now offers a "model" config
// option: the choices come from MEMDOOR_ACP_MODELS, picking one pins it for
// that conversation only (keep:false — another session's default stays), and
// "default" hands the conversation back to the gateway's own model.

// acpModel is one choice of the model option.
type acpModel struct {
	Value string // the pin sent to the gateway: provider:model
	Name  string // what the editor shows
}

const (
	acpModelOption  = acp.SessionConfigId("model")
	acpModelDefault = acp.SessionConfigValueId("default")
)

// acpModelsFromEnv reads MEMDOOR_ACP_MODELS: "pin|Name" entries separated by
// ";" or newlines (a pin may hold ":" and "/", never "|" or ";"). An entry
// without a name shows its pin. MEMDOOR_ACP_DEFAULT_NAME names the default.
func acpModelsFromEnv() (models []acpModel, defaultName string) {
	for _, entry := range strings.FieldsFunc(os.Getenv("MEMDOOR_ACP_MODELS"), func(r rune) bool { return r == ';' || r == '\n' }) {
		pin, name, _ := strings.Cut(entry, "|")
		pin, name = strings.TrimSpace(pin), strings.TrimSpace(name)
		if pin == "" || acp.SessionConfigValueId(pin) == acpModelDefault {
			continue
		}
		if name == "" {
			name = pin
		}
		models = append(models, acpModel{Value: pin, Name: name})
	}
	return models, strings.TrimSpace(os.Getenv("MEMDOOR_ACP_DEFAULT_NAME"))
}

// modelOptions is the session's config: the model option with its current
// value, or nothing when no choices are configured.
func (a *acpAgent) modelOptions(current acp.SessionConfigValueId) []acp.SessionConfigOption {
	if len(a.models) == 0 {
		return nil
	}
	defaultName := "Default"
	if a.defaultModelName != "" {
		defaultName = "Default (" + a.defaultModelName + ")"
	}
	opts := acp.SessionConfigSelectOptionsUngrouped{{Name: defaultName, Value: acpModelDefault}}
	for _, m := range a.models {
		opts = append(opts, acp.SessionConfigSelectOption{Name: m.Name, Value: acp.SessionConfigValueId(m.Value)})
	}
	category := acp.SessionConfigOptionCategoryModel
	if current == "" {
		current = acpModelDefault
	}
	return []acp.SessionConfigOption{{Select: &acp.SessionConfigOptionSelect{
		Type: "select", Id: acpModelOption, Name: "Model", Category: &category,
		CurrentValue: current, Options: acp.SessionConfigSelectOptions{Ungrouped: &opts},
	}}}
}

// SetSessionConfigOption pins the chosen model on the session's conversation.
func (a *acpAgent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	v := params.ValueId
	if v == nil || v.ConfigId != acpModelOption || len(a.models) == 0 {
		return acp.SetSessionConfigOptionResponse{}, acp.NewInvalidParams(map[string]any{"error": "the only option is model"})
	}
	pin := ""
	if v.Value != acpModelDefault {
		for _, m := range a.models {
			if acp.SessionConfigValueId(m.Value) == v.Value {
				pin = m.Value
			}
		}
		if pin == "" {
			return acp.SetSessionConfigOptionResponse{}, acp.NewInvalidParams(map[string]any{"error": fmt.Sprintf("no model %q", v.Value)})
		}
	}
	a.mu.Lock()
	s, ok := a.sessions[v.SessionId]
	a.mu.Unlock()
	if !ok {
		return acp.SetSessionConfigOptionResponse{}, fmt.Errorf("no session %s: session/new first", v.SessionId)
	}
	if err := a.runner.SetModel(ctx, s.conversation, pin); err != nil {
		return acp.SetSessionConfigOptionResponse{}, err
	}
	a.mu.Lock()
	s.model = v.Value
	a.mu.Unlock()
	return acp.SetSessionConfigOptionResponse{ConfigOptions: a.modelOptions(v.Value)}, nil
}

// SetModel pins model on the conversation for this conversation only; ""
// hands it back to the gateway's model.
func (r *gatewayRunner) SetModel(ctx context.Context, conversation, model string) error {
	body := map[string]interface{}{"workspace": r.workspace, "channel_id": conversation, "agent": r.agentName(), "keep": false}
	if model == "" {
		body["tier"] = "auto"
	} else {
		body["model"] = model
	}
	var out map[string]interface{}
	if err := r.c.PostJSON("/api/route", body, &out); err != nil {
		return fmt.Errorf("could not set the model to %q: %w", model, err)
	}
	return nil
}
