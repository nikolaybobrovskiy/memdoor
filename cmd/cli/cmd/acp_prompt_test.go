package cmd

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

// The editor offers attachments only for what initialize declares.
func TestInitializeDeclaresPicturesAndEmbeddedContext(t *testing.T) {
	resp, err := newACPAgent(nil).Initialize(context.Background(), acp.InitializeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	pc := resp.AgentCapabilities.PromptCapabilities
	if !pc.Image || !pc.EmbeddedContext {
		t.Fatalf("prompt capabilities: %+v", pc)
	}
}

func fileURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x -> /C:/x
	}
	return "file://" + p
}

// Every attachment Rider can send becomes what the window makes of it.
func TestAttachmentsBecomeMentionsSavedPicturesAndQuotes(t *testing.T) {
	cwd := t.TempDir()
	src := filepath.Join(cwd, "src", "App.vue")
	png := []byte("\x89PNG\r\n\x1a\nfake-picture")
	blob := base64.StdEncoding.EncodeToString(png)
	mimePNG, mimePDF := "image/png", "application/pdf"
	now := time.Date(2026, 11, 2, 23, 59, 1, 0, time.UTC)

	got := promptTextAt(cwd, []acp.ContentBlock{
		acp.TextBlock("fix the header"),
		acp.ResourceLinkBlock("App.vue", fileURI(src)),
		{Image: &acp.ContentBlockImage{Type: "image", Data: blob, MimeType: "image/png"}},
		{Resource: &acp.ContentBlockResource{Type: "resource", Resource: acp.EmbeddedResourceResource{
			TextResourceContents: &acp.TextResourceContents{Uri: fileURI(src), Text: "<template>\n</template>\n"}}}},
		{Resource: &acp.ContentBlockResource{Type: "resource", Resource: acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{Uri: "file:///clip.png", Blob: blob, MimeType: &mimePNG}}}},
		{Resource: &acp.ContentBlockResource{Type: "resource", Resource: acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{Uri: fileURI(filepath.Join(cwd, "spec.pdf")), Blob: "AAAA", MimeType: &mimePDF}}}},
		{Image: &acp.ContentBlockImage{Type: "image", Data: blob, MimeType: "image/tiff"}},
	}, now)

	pics, _ := filepath.Glob(filepath.Join(cwd, ".memdoor", "paste-20261102-235901-*.png"))
	if len(pics) != 2 {
		t.Fatalf("saved pictures: %v", pics)
	}
	pic1, pic2 := pics[0], pics[1] // -1-… sorts before -2-…
	want := []string{
		"fix the header",
		"@" + src,
		"@" + pic1,
		"@" + src + "\n```\n<template>\n</template>\n```",
		"@" + pic2,
		"@" + filepath.Join(cwd, "spec.pdf"),
		"(an attached image/tiff picture was left out: only png, jpeg, gif and webp are kept)",
	}
	if got != strings.Join(want, "\n") {
		t.Fatalf("prompt:\n%s\n--- want ---\n%s", got, strings.Join(want, "\n"))
	}
	for _, p := range []string{pic1, pic2} {
		if b, err := os.ReadFile(p); err != nil || string(b) != string(png) {
			t.Fatalf("saved picture %s: %q %v", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cwd, ".memdoor", ".gitignore")); err != nil {
		t.Fatalf("pictures stay out of commits: %v", err)
	}
}

// A picture that cannot be decoded is said, not silently dropped.
func TestAnUnreadablePictureIsSaid(t *testing.T) {
	got := promptTextAt(t.TempDir(), []acp.ContentBlock{
		{Image: &acp.ContentBlockImage{Type: "image", Data: "%%%", MimeType: "image/png"}},
	}, time.Now())
	if got != "(an attached picture could not be read)" {
		t.Fatalf("%q", got)
	}
}

func TestAttachmentPathReadsFileURIs(t *testing.T) {
	if got := attachmentPath("https://example.com/x", "x"); got != "https://example.com/x" {
		t.Errorf("a web link stays a link: %q", got)
	}
	if got := attachmentPath("", "notes.md"); got != "notes.md" {
		t.Errorf("no URI: the name: %q", got)
	}
	if runtime.GOOS == "windows" {
		if got := attachmentPath("file:///C:/Users/me/My%20App/a.cs", ""); got != `C:\Users\me\My App\a.cs` {
			t.Errorf("a Windows file URI: %q", got)
		}
	} else if got := attachmentPath("file:///home/me/a.go", ""); got != "/home/me/a.go" {
		t.Errorf("a file URI: %q", got)
	}
}

// Two prompts in the same second keep both pictures: names never collide.
func TestPicturesSavedInTheSameSecondDoNotOverwrite(t *testing.T) {
	cwd := t.TempDir()
	now := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	img := func(s string) []acp.ContentBlock {
		return []acp.ContentBlock{{Image: &acp.ContentBlockImage{Type: "image", MimeType: "image/png",
			Data: base64.StdEncoding.EncodeToString([]byte(s))}}}
	}
	a := strings.TrimPrefix(promptTextAt(cwd, img("first"), now), "@")
	b := strings.TrimPrefix(promptTextAt(cwd, img("second"), now), "@")
	if a == b {
		t.Fatalf("same name %s", a)
	}
	if got, _ := os.ReadFile(a); string(got) != "first" {
		t.Fatalf("the first picture was overwritten: %q", got)
	}
}
