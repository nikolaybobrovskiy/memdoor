package providers

import (
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// A picture in a user message reaches an OpenAI-compatible vendor as an
// image_url part; a message without one keeps plain string content.
func TestAPictureBecomesAnImageURLPart(t *testing.T) {
	msgs := convertAnthropicToOAI(llm.MessageNewParams{Messages: []llm.MessageParam{
		llm.NewUserMessage(
			llm.ContentBlockParamUnion{OfImage: &llm.ImageBlockParam{Type: "image", MediaType: "image/png", Data: "QUJD"}},
			llm.NewTextBlock("what is this?")),
		llm.NewUserMessage(llm.NewTextBlock("plain")),
	}})
	if len(msgs) != 2 {
		t.Fatalf("%d messages", len(msgs))
	}
	raw, _ := json.Marshal(msgs)
	got := string(raw)
	want := `[{"role":"user","content":[{"text":"what is this?","type":"text"},{"image_url":{"url":"data:image/png;base64,QUJD"},"type":"image_url"}]},{"role":"user","content":"plain"}]`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if strings.Count(got, `"content"`) != 2 {
		t.Fatal("one content key per message")
	}
}
