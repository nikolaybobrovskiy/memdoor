package gateway

import (
	"encoding/json"
	"runtime"
	"testing"
)

// On Windows a Git Bash path the model copies from bash output (/i/x) is the
// drive path it names: rewritten to I:/x rather than joined under the workdir,
// and fenced like any absolute path.
func TestAShellDrivePathIsTheDrivePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash drive paths exist on Windows only")
	}
	wd := `I:\Projects\app`
	out := confineToCoderWorkdir("read_file", json.RawMessage(`{"path":"/i/Projects/app/src/Program.cs"}`), wd)
	var in map[string]any
	if err := json.Unmarshal(out, &in); err != nil {
		t.Fatal(err)
	}
	if in["path"] != "I:/Projects/app/src/Program.cs" {
		t.Fatalf("path: %v", in["path"])
	}
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/i/Projects/app/src/x.cs"}`), wd); msg != "" {
		t.Fatalf("inside the workdir is allowed: %s", msg)
	}
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/c/Windows/x.cs"}`), wd); msg == "" {
		t.Fatal("a shell drive path outside the workdir must be refused like any absolute path")
	}
}

// A lowercase-drive workdir accepts its own files written the Git Bash way,
// and verify takes a Git Bash dir as the drive's.
func TestAShellDrivePathInALowercaseWorkdir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash drive paths exist on Windows only")
	}
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/i/Projects/app/x.txt"}`), `i:\Projects\app`); msg != "" {
		t.Fatalf("inside a lowercase-drive workdir: %s", msg)
	}
	out := confineToCoderWorkdir("verify", json.RawMessage(`{"dir":"/i/Projects/app/sub"}`), `I:\Projects\app`)
	var in map[string]any
	_ = json.Unmarshal(out, &in)
	if in["dir"] != "I:/Projects/app/sub" {
		t.Fatalf("verify dir: %v", in["dir"])
	}
}
