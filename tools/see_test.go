package tools

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeeShowsThePictureToTheSeer(t *testing.T) {
	dir := t.TempDir()
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	p := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(p, png, 0o644); err != nil {
		t.Fatal(err)
	}
	var gotAgent, gotType, gotData, gotQ string
	SetSeer(func(agent, mediaType, data, question string) (string, error) {
		gotAgent, gotType, gotData, gotQ = agent, mediaType, data, question
		return "a red header", nil
	})
	t.Cleanup(func() { SetSeer(nil) })

	in, _ := json.Marshal(map[string]string{"path": "@" + p, "question": " what colour? ", "agent": "coder"})
	out, err := See(in)
	if err != nil || out != "a red header" {
		t.Fatalf("%q %v", out, err)
	}
	if gotAgent != "coder" || gotType != "image/png" || gotQ != "what colour?" || gotData != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("seer got %q %q %q %q", gotAgent, gotType, gotQ, gotData)
	}
}

func TestSeeRefusesWhatIsNotAPicture(t *testing.T) {
	SetSeer(func(string, string, string, string) (string, error) { t.Fatal("not a picture"); return "", nil })
	t.Cleanup(func() { SetSeer(nil) })
	p := filepath.Join(t.TempDir(), "notes.png")
	_ = os.WriteFile(p, []byte("just text"), 0o644)
	in, _ := json.Marshal(map[string]string{"path": p})
	if _, err := See(in); err == nil || !strings.Contains(err.Error(), "not a png") {
		t.Fatalf("%v", err)
	}
	in, _ = json.Marshal(map[string]string{"path": filepath.Join(t.TempDir(), "missing.png")})
	if _, err := See(in); err == nil {
		t.Fatal("a missing file is an error")
	}
}
