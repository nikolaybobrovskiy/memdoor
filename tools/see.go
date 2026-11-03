package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"memdoor/pkg/shared"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// see — the coder looks at a picture (2026-11-03). The coder's prompt has told
// it since the start to look at a pasted @…png "with see", and no see tool
// existed: the coder decoded the PNG's pixels with Python in bash instead,
// which answers "what colour" and nothing a screenshot is pasted for. see
// shows the picture to the answering model in a one-off call and returns
// what it saw, so the conversation itself stays text.

type SeeInput struct {
	Path     string `json:"path" jsonschema_description:"The picture's path (png, jpeg, gif or webp): an @…png the person pasted, or a screenshot in the project."`
	Question string `json:"question,omitempty" jsonschema_description:"Optional. What to look for in the picture; without it, the picture is described in full."`
	// Call and Workdir are set by the harness, never by the model: the call
	// names the turn the look belongs to (its model, cancellation and meter),
	// and a relative path is the project's, not the gateway's.
	Call    string `json:"call,omitempty" jsonschema:"-"`
	Workdir string `json:"workdir,omitempty" jsonschema:"-"`
}

var seeSchema = GenerateSchema[SeeInput]()

var SeeDefinition = ToolDefinition{
	Name: "see",
	Description: "Look at a picture: a screenshot the person pasted (an @…png in the request) or any png, jpeg, gif or webp file. " +
		"Returns what is in it — layout, text, colours, errors shown — answering your question when you give one.",
	InputSchema: seeSchema,
	Function:    See,
}

// SeeMaxBytes is the largest picture see sends: the model APIs refuse more.
const SeeMaxBytes = 5 << 20

// seer shows a picture to the answering model of the turn that made call;
// wired at startup.
var seer func(call, mediaType, data, question string) (string, error)

// SetSeer wires who looks at the pictures.
func SetSeer(fn func(call, mediaType, data, question string) (string, error)) { seer = fn }

var seeMediaTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

func See(input json.RawMessage) (string, error) {
	var in SeeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	path := shared.NativePath(strings.TrimPrefix(strings.TrimSpace(in.Path), "@"))
	if path == "" {
		return "", fmt.Errorf("see: give the picture's path")
	}
	if !filepath.IsAbs(path) && in.Workdir != "" {
		path = filepath.Join(in.Workdir, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	// Read one byte past the limit and no more: a huge file is refused
	// without being loaded.
	raw, err := io.ReadAll(io.LimitReader(f, SeeMaxBytes+1))
	f.Close()
	if err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	if len(raw) > SeeMaxBytes {
		return "", fmt.Errorf("see: %s is over 5 MB; pictures that large are not sent — shrink it first", path)
	}
	mediaType := http.DetectContentType(raw)
	switch {
	case !seeMediaTypes[mediaType]:
		return "", fmt.Errorf("see: %s is %s, not a png, jpeg, gif or webp picture", path, mediaType)
	case seer == nil:
		return "", fmt.Errorf("see: no model to look with")
	}
	return seer(in.Call, mediaType, base64.StdEncoding.EncodeToString(raw), strings.TrimSpace(in.Question))
}
