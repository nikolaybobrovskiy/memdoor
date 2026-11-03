package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A patch header naming a Git Bash path on Windows edits that drive's file.
func TestAPatchHeaderWithAShellDrivePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash drive paths exist on Windows only")
	}
	hunks, err := parsePatch("*** Begin Patch\n*** Update File: /i/Projects/app/Program.cs\n@@\n-a\n+b\n*** End Patch")
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 || hunks[0].path != "I:/Projects/app/Program.cs" {
		t.Fatalf("%+v", hunks)
	}
}

// shellPath writes a Windows path the Git Bash way: C:\x\y -> /c/x/y.
func shellPath(p string) string {
	p = filepath.ToSlash(p)
	return "/" + strings.ToLower(p[:1]) + p[2:]
}

func applyIn(t *testing.T, cwd, patch string) (string, error) {
	t.Helper()
	in, _ := json.Marshal(map[string]string{"input": patch, "cwd": cwd})
	return ApplyPatch(in)
}

// Every file a patch names stays inside the project folder: a drive path
// outside it, written natively or the Git Bash way, and a move out of it are
// refused before anything is written.
func TestAPatchNeverWritesOutsideTheProject(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash drive paths exist on Windows only")
	}
	root := t.TempDir()
	cwd := filepath.Join(root, "app")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{cwd, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inside := filepath.Join(cwd, "x.txt")
	_ = os.WriteFile(inside, []byte("a\n"), 0o644)

	for name, patch := range map[string]string{
		"native add": "*** Begin Patch\n*** Add File: " + filepath.ToSlash(filepath.Join(outside, "n.txt")) + "\n+x\n*** End Patch",
		"shell add":  "*** Begin Patch\n*** Add File: " + shellPath(filepath.Join(outside, "s.txt")) + "\n+x\n*** End Patch",
		"move out":   "*** Begin Patch\n*** Update File: x.txt\n*** Move to: " + shellPath(filepath.Join(outside, "m.txt")) + "\n@@\n-a\n+b\n*** End Patch",
	} {
		if _, err := applyIn(t, cwd, patch); err == nil || !strings.Contains(err.Error(), "outside the project folder") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if left, _ := os.ReadDir(outside); len(left) != 0 {
		t.Fatalf("written outside the project: %v", left)
	}
	if b, _ := os.ReadFile(inside); string(b) != "a\n" {
		t.Fatalf("the refused move changed the file: %q", b)
	}

	// Inside the project, the Git Bash spelling edits the real file.
	if _, err := applyIn(t, cwd, "*** Begin Patch\n*** Update File: "+shellPath(inside)+"\n@@\n-a\n+b\n*** End Patch"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(inside); strings.TrimSpace(string(b)) != "b" {
		t.Fatalf("not edited: %q", b)
	}
}

// A line-anchored patch header written the Git Bash way edits the real file.
func TestAHashlineHeaderWithAShellDrivePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash drive paths exist on Windows only")
	}
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	if _, err := applyHL(t, dir, "["+shellPath(path)+"#"+FileTag(hlSrc)+"]\nreplace 5-5 \"return a\":\n+\treturn a + b\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), "return a + b") {
		t.Fatalf("not edited:\n%s", got)
	}
}
