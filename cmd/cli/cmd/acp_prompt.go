package cmd

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coder/acp-go-sdk"

	"memdoor/pkg/shared"
	"memdoor/pkg/workflow"
)

// WHAT AN EDITOR ATTACHES (2026-11-02, JetBrains Rider). The editor offers file
// pickers and image paste only when initialize declares the image and
// embeddedContext prompt capabilities, and this adapter declared neither, so
// Rider could not attach anything; promptText also dropped every block that
// was not text. An attachment now becomes what the window makes of one (the
// coder's prompt, gateway/agent_seeding.go): a file is an @mention, a pasted
// picture is saved under the project's .memdoor folder (ignored by git, as
// cmd/tui/ui/paste.go does) and mentioned as @…png for the coder to look at
// with see, and an embedded excerpt is quoted with the file it came from.

// acpPromptCapabilities is what the adapter accepts in session/prompt.
var acpPromptCapabilities = acp.PromptCapabilities{Image: true, EmbeddedContext: true}

// imageExtensions names the file a pasted picture is saved as, by MIME type.
var imageExtensions = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/jpg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
}

// promptText turns the prompt's blocks into the one request the coder reads.
// cwd is the session's project directory, where pasted pictures are kept.
func promptText(cwd string, blocks []acp.ContentBlock) string {
	return promptTextAt(cwd, blocks, time.Now())
}

func promptTextAt(cwd string, blocks []acp.ContentBlock, now time.Time) string {
	var parts []string
	saved := 0
	for _, c := range blocks {
		switch {
		case c.Text != nil:
			parts = append(parts, c.Text.Text)
		case c.ResourceLink != nil:
			parts = append(parts, "@"+attachmentPath(c.ResourceLink.Uri, c.ResourceLink.Name))
		case c.Image != nil:
			saved++
			parts = append(parts, savedPicture(cwd, c.Image.Data, c.Image.MimeType, now, saved))
		case c.Resource != nil:
			r := c.Resource.Resource
			switch {
			case r.TextResourceContents != nil:
				t := r.TextResourceContents
				parts = append(parts, fmt.Sprintf("@%s\n```\n%s\n```", attachmentPath(t.Uri, ""), strings.TrimRight(t.Text, "\n")))
			case r.BlobResourceContents != nil:
				b := r.BlobResourceContents
				mime := ""
				if b.MimeType != nil {
					mime = *b.MimeType
				}
				if _, ok := imageExtensions[strings.ToLower(mime)]; ok {
					saved++
					parts = append(parts, savedPicture(cwd, b.Blob, mime, now, saved))
				} else {
					parts = append(parts, "@"+attachmentPath(b.Uri, ""))
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// attachmentPath is a resource's local path when its URI is a file: URI, else
// the URI itself (or its name when there is no URI).
func attachmentPath(uri, name string) string {
	if uri == "" {
		return name
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" { // file://server/share/x: a UNC path
		p = "//" + u.Host + p
	}
	// file:///C:/x parses to /C:/x; the drive letter starts the Windows path.
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// savedPicture writes a base64 picture to <cwd>/.memdoor/paste-<time>-<n>.<ext>
// and returns its @mention, or a line saying why it could not be kept.
func savedPicture(cwd, data, mime string, now time.Time, n int) string {
	ext, ok := imageExtensions[strings.ToLower(mime)]
	if !ok {
		return fmt.Sprintf("(an attached %s picture was left out: only png, jpeg, gif and webp are kept)", mime)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(raw) == 0 {
		return "(an attached picture could not be read)"
	}
	dir := filepath.Join(cwd, shared.MemdoorDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("(an attached picture could not be saved: %v)", err)
	}
	// The picture stays out of commits; .memdoor/workflows stays in them.
	workflow.EnsureIgnored(cwd)
	target := filepath.Join(dir, fmt.Sprintf("paste-%s-%d%s", now.Format("20060102-150405"), n, ext))
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		return fmt.Sprintf("(an attached picture could not be saved: %v)", err)
	}
	return "@" + target
}
