package gateway

import (
	"context"
	"strings"
	"testing"
)

// A see call is bound to its turn while it runs and unbound after: the look
// inherits the turn's context (model pin, cancellation, meter) and a stale
// call name reaches nothing.
func TestASeeCallRunsInItsTurnsContext(t *testing.T) {
	ar := &AgentRuntime{}
	turn, cancel := context.WithCancel(context.WithValue(context.Background(), "buddy_agent_name", "coder"))
	call := ar.bindSeeCall(turn)
	v, ok := ar.seeCalls.Load(call)
	if !ok {
		t.Fatal("call not bound")
	}
	ctx := v.(context.Context)
	if name, _ := ctx.Value("buddy_agent_name").(string); name != "coder" {
		t.Fatalf("agent %q", name)
	}
	cancel()
	if ctx.Err() == nil {
		t.Fatal("cancelling the turn must cancel the look")
	}
	ar.seeCalls.Delete(call)
	ar.clientFactory = nil
	if _, err := ar.seePicture(call, "image/png", "", ""); err == nil {
		t.Fatal("no client and no binding is an error")
	}
	if a, b := ar.bindSeeCall(turn), ar.bindSeeCall(turn); a == b || !strings.HasPrefix(a, "see-") {
		t.Fatalf("call names %q %q", a, b)
	}
}
