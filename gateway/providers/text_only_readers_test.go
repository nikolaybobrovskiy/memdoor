package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every reader that copies a provider-stated input list marks a text-only
// model, and leaves a vision model and an unlisted one able to see.
func TestEveryStatedInputListMarksTextOnly(t *testing.T) {
	clearProviderEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/language-models"): // xAI's modalities endpoint
			_, _ = io.WriteString(w, `{"models":[{"id":"text-model","input_modalities":["text"]},{"id":"vision-model","input_modalities":["text","image"]}]}`)
		default:
			_, _ = io.WriteString(w, `{"data":[
			  {"id":"text-model","active":true,"context_window":1000,"context_length":1000,"input_modalities":["text"],"supported_features":["tools"]},
			  {"id":"vision-model","active":true,"context_window":1000,"context_length":1000,"input_modalities":["text","image"],"supported_features":["tools"]},
			  {"id":"unlisted-model","active":true,"context_window":1000,"context_length":1000,"supported_features":["tools"]}]}`)
		}
	}))
	defer srv.Close()

	for name, r := range map[string]ModelReader{"groq": groqReader{}, "xai": xaiReader{}, "baseten": basetenReader{}, "deepseek": deepseekReader{}} {
		list, err := r.ListModels(context.Background(), Provider{ID: name, Base: srv.URL + "/v1", Key: "k"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := map[string]Model{}
		for _, m := range list {
			got[m.ID] = m
		}
		if m, ok := got["text-model"]; !ok || TakesPictures(m) {
			t.Errorf("%s: a stated text-only model must refuse pictures: %+v", name, m)
		}
		if m, ok := got["vision-model"]; !ok || !TakesPictures(m) {
			t.Errorf("%s: a vision model must see: %+v", name, m)
		}
		if m, ok := got["unlisted-model"]; ok && !TakesPictures(m) {
			t.Errorf("%s: an unlisted model is not text-only: %+v", name, m)
		}
	}
}
