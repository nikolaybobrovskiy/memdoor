package shared

import "testing"

func TestNativePathTurnsAShellDrivePathIntoWindows(t *testing.T) {
	for in, want := range map[string]string{
		"/i/Projects/app/Program.cs": "I:/Projects/app/Program.cs",
		"/c":                         "C:/",
		"/c/":                        "C:/",
		"/cygdrive/d/src/x.go":       "D:/src/x.go",
		"/mnt/e/x":                   "E:/x",
		"/home/me/x":                 "/home/me/x", // not a drive letter
		"/ab/x":                      "/ab/x",
		"src/x.cs":                   "src/x.cs",
		"I:/Projects/x.cs":           "I:/Projects/x.cs",
		`I:\Projects\x.cs`:           `I:\Projects\x.cs`,
		"":                           "",
		"/":                          "/",
		"/1/x":                       "/1/x",
	} {
		if got := nativePath("windows", in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if got := nativePath("linux", "/i/x"); got != "/i/x" {
		t.Errorf("elsewhere a path is left alone: %q", got)
	}
}
