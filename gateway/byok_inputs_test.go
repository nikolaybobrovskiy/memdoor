package gateway

import (
	"testing"
	"time"
)

// OpenRouter's own catalogue carries each model's listed inputs, so a model
// it states is text-only refuses pictures and the rest do not.
func TestTheByokCatalogueCarriesTextOnlyInputs(t *testing.T) {
	theByokCatalog.mu.Lock()
	saved, savedAt := theByokCatalog.list, theByokCatalog.at
	theByokCatalog.list = []byokModel{
		{ID: "cohere/north-mini-code:free", Tools: true, Inputs: []string{"text"}},
		{ID: "z-ai/glm-5.3v", Tools: true, Inputs: []string{"text", "image"}},
		{ID: "unlisted", Tools: true},
	}
	theByokCatalog.at = time.Now()
	theByokCatalog.mu.Unlock()
	t.Cleanup(func() {
		theByokCatalog.mu.Lock()
		theByokCatalog.list, theByokCatalog.at = saved, savedAt
		theByokCatalog.mu.Unlock()
	})

	list, err := byokModels("k")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"cohere/north-mini-code:free": true, "z-ai/glm-5.3v": false, "unlisted": false}
	for _, m := range list {
		if m.TextOnly != want[m.ID] {
			t.Errorf("%s: TextOnly %v", m.ID, m.TextOnly)
		}
	}
}
