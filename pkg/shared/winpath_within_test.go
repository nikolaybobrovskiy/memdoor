package shared

import (
	"runtime"
	"testing"
)

func TestWithinIgnoresTheDrivesCaseOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive letters exist on Windows only")
	}
	for _, c := range []struct {
		root, p string
		want    bool
	}{
		{`i:\Projects\app`, `I:/Projects/app/x.txt`, true},
		{`I:\Projects\app`, `i:\projects\app\x.txt`, true},
		{`I:\Projects\app`, `I:\Projects\app`, true},
		{`I:\Projects\app`, `I:\Projects\app2\x`, false},
		{`I:\Projects\app`, `I:\Projects\x`, false},
		{`I:\Projects\app`, `C:\Projects\app\x`, false},
	} {
		if got := Within(c.root, c.p); got != c.want {
			t.Errorf("Within(%q, %q) = %v", c.root, c.p, got)
		}
	}
}
