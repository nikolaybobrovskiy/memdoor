package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A relay serving DeepSeek over chat completions (OpenCode Go) lists the
// models.dev shape: the window and the cap under "limit" (shape copied from
// the live answer of 2026-11-02). DeepSeek's own fields still win.
func TestTheDeepSeekReaderReadsARelaysLimits(t *testing.T) {
	clearProviderEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"object":"list","data":[
		  {"id":"deepseek-v4.1-flash","name":"DeepSeek V4.1 Flash","type":"chat","limit":{"context":1000000,"output":384000},
		   "capabilities":{"tools":{"supported":true}}},
		  {"id":"deepseek-native","context_window":131072,"max_output_tokens":8192,"limit":{"context":1,"output":1}},
		  {"id":"deepseek-bare"}]}`)
	}))
	defer srv.Close()

	list, err := deepseekReader{}.ListModels(context.Background(), Provider{ID: "deepseek", Base: srv.URL + "/v1", Key: "k"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Model{}
	for _, m := range list {
		got[m.ID] = m
	}
	if m := got["deepseek-v4.1-flash"]; m.Context != 1_000_000 || m.MaxOutput != 384_000 {
		t.Errorf("the relay's limits: %+v", m)
	}
	if m := got["deepseek-native"]; m.Context != 131072 || m.MaxOutput != 8192 {
		t.Errorf("DeepSeek's own fields win: %+v", m)
	}
	if m := got["deepseek-bare"]; m.Context != 0 || m.MaxOutput != 0 {
		t.Errorf("nothing stated stays unknown: %+v", m)
	}
}
