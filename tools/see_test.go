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
	var gotCall, gotType, gotData, gotQ string
	SetSeer(func(call, mediaType, data, question string) (string, error) {
		gotCall, gotType, gotData, gotQ = call, mediaType, data, question
		return "a red header", nil
	})
	t.Cleanup(func() { SetSeer(nil) })

	in, _ := json.Marshal(map[string]string{"path": "@" + p, "question": " what colour? ", "call": "see-7"})
	out, err := See(in)
	if err != nil || out != "a red header" {
		t.Fatalf("%q %v", out, err)
	}
	if gotCall != "see-7" || gotType != "image/png" || gotQ != "what colour?" || gotData != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("seer got %q %q %q %q", gotCall, gotType, gotQ, gotData)
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

// A relative path is the project's: the harness passes the workdir.
func TestSeeReadsARelativePathInTheProject(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n\x1a\nproject"), 0o644)
	var got string
	SetSeer(func(_, _, data, _ string) (string, error) { got = data; return "ok", nil })
	t.Cleanup(func() { SetSeer(nil) })
	in, _ := json.Marshal(map[string]string{"path": "@shot.png", "workdir": dir})
	if _, err := See(in); err != nil {
		t.Fatal(err)
	}
	if raw, _ := base64.StdEncoding.DecodeString(got); !strings.HasSuffix(string(raw), "project") {
		t.Fatalf("read the wrong file: %q", raw)
	}
}

func TestSeeRefusesAPictureOverTheLimit(t *testing.T) {
	SetSeer(func(string, string, string, string) (string, error) { t.Fatal("too big"); return "", nil })
	t.Cleanup(func() { SetSeer(nil) })
	p := filepath.Join(t.TempDir(), "big.png")
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, SeeMaxBytes)...)
	_ = os.WriteFile(p, big, 0o644)
	in, _ := json.Marshal(map[string]string{"path": p})
	if _, err := See(in); err == nil || !strings.Contains(err.Error(), "over 5 MB") {
		t.Fatalf("%v", err)
	}
}
