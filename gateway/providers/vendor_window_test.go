package providers

import "testing"

func TestDeclaredVendorWindowIsTheEnginesModelOnly(t *testing.T) {
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "256000")
	re := &RemoteEngine{Vendor: VendorDeepSeek, Model: "cohere/north-mini-code:free"}
	if w := DeclaredVendorWindow(re, "deepseek", "cohere/north-mini-code:free"); w != 256000 {
		t.Fatalf("declared window: %d", w)
	}
	if w := DeclaredVendorWindow(re, "deepseek", "other"); w != 0 {
		t.Fatalf("another model: %d", w)
	}
	if w := DeclaredVendorWindow(re, "groq", "cohere/north-mini-code:free"); w != 0 {
		t.Fatalf("the same id on another provider: %d", w)
	}
	if w := DeclaredVendorWindow(&RemoteEngine{Vendor: VendorGoogle, Model: "g"}, "gemini", "g"); w != 256000 {
		t.Fatalf("google is listed as gemini: %d", w)
	}
	if w := DeclaredVendorWindow(&RemoteEngine{Model: "cohere/north-mini-code:free"}, "deepseek", "cohere/north-mini-code:free"); w != 0 {
		t.Fatalf("not a vendor engine: %d", w)
	}
	if w := DeclaredVendorWindow(nil, "deepseek", "x"); w != 0 {
		t.Fatalf("no engine: %d", w)
	}
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	if w := DeclaredVendorWindow(re, "deepseek", "cohere/north-mini-code:free"); w != 0 {
		t.Fatalf("nothing declared: %d", w)
	}
}

// Only an explicit text-only list refuses pictures: a reader's placeholder
// ["text"] (Gemini, Anthropic without capabilities) leaves TextOnly false.
func TestOnlyAnExplicitTextOnlyListRefusesPictures(t *testing.T) {
	for _, c := range []struct {
		in       []string
		textOnly bool
	}{
		{nil, false},
		{[]string{"text"}, true},
		{[]string{"text", "image"}, false},
		{[]string{"text", "image/png"}, false},
	} {
		if got := ExplicitTextOnly(c.in); got != c.textOnly {
			t.Errorf("%v: %v", c.in, got)
		}
	}
	if !TakesPictures(Model{Inputs: []string{"text"}}) {
		t.Error("a placeholder input list must not refuse pictures")
	}
	if TakesPictures(Model{Inputs: []string{"text"}, TextOnly: true}) {
		t.Error("an explicit text-only model must refuse pictures")
	}
}
