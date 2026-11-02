package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every picture the editor adapter saves stays out of commits, in a fresh
// .gitignore and in one an older release wrote.
func TestEveryPastedPictureTypeIsIgnored(t *testing.T) {
	for _, existing := range []string{"", "runs/\npaste-*.png\n"} {
		dir := t.TempDir()
		if existing != "" {
			_ = os.MkdirAll(filepath.Join(dir, ".memdoor"), 0o755)
			_ = os.WriteFile(filepath.Join(dir, ".memdoor", ".gitignore"), []byte(existing), 0o644)
		}
		EnsureIgnored(dir)
		b, _ := os.ReadFile(filepath.Join(dir, ".memdoor", ".gitignore"))
		for _, ext := range []string{"png", "jpg", "gif", "webp"} {
			if !strings.Contains(string(b), "paste-*."+ext+"\n") {
				t.Errorf("existing %q: paste-*.%s not ignored:\n%s", existing, ext, b)
			}
		}
	}
}
