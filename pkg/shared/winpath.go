package shared

import (
	"path/filepath"
	"runtime"
	"strings"
)

// NATIVE PATHS FROM A POSIX SHELL (2026-11-03). On Windows the coder's bash
// is Git Bash, so pwd, ls and find print /i/Projects/app/x.cs, and the model
// copies that into apply_patch or read_file. Windows does not take it as
// absolute, so it was joined under the workdir as
// I:\Projects\app\i\Projects\app\x.cs and the call failed again and again.

// NativePath turns a drive path written the POSIX-shell way (/i/x, /cygdrive/i/x,
// /mnt/i/x) into the Windows form (I:/x) on Windows; any other path, and every
// path elsewhere, is returned unchanged.
func NativePath(p string) string { return nativePath(runtime.GOOS, p) }

func nativePath(goos, p string) string {
	if goos != "windows" {
		return p
	}
	rest := p
	for _, prefix := range []string{"/cygdrive/", "/mnt/"} {
		if len(rest) > len(prefix) && rest[:len(prefix)] == prefix {
			rest = rest[len(prefix)-1:]
			break
		}
	}
	// /x or /x/... with a single letter: a drive.
	if len(rest) < 2 || rest[0] != '/' || !isLetter(rest[1]) || (len(rest) > 2 && rest[2] != '/') {
		return p
	}
	drive := rest[1]
	if drive >= 'a' {
		drive -= 'a' - 'A'
	}
	if len(rest) == 2 {
		return string(drive) + ":/"
	}
	return string(drive) + ":" + rest[2:]
}

// Within reports whether p is root or inside it. filepath.Rel compares
// Windows volumes and names case-insensitively, so i:\app and I:/app/x agree.
func Within(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
