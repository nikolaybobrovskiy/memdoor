package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// Agent is set by the harness, never by the model.
	Agent string `json:"agent,omitempty" jsonschema:"-"`
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

// seer shows a picture to the answering model; wired at startup.
var seer func(agent, mediaType, data, question string) (string, error)

// SetSeer wires who looks at the pictures.
func SetSeer(fn func(agent, mediaType, data, question string) (string, error)) { seer = fn }

var seeMediaTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

func See(input json.RawMessage) (string, error) {
	var in SeeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	path := strings.TrimPrefix(strings.TrimSpace(in.Path), "@")
	if path == "" {
		return "", fmt.Errorf("see: give the picture's path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	mediaType := http.DetectContentType(raw)
	switch {
	case !seeMediaTypes[mediaType]:
		return "", fmt.Errorf("see: %s is %s, not a png, jpeg, gif or webp picture", path, mediaType)
	case len(raw) > SeeMaxBytes:
		return "", fmt.Errorf("see: %s is %d MB; pictures over 5 MB are not sent — shrink it first", path, len(raw)>>20)
	case seer == nil:
		return "", fmt.Errorf("see: no model to look with")
	}
	return seer(in.Agent, mediaType, base64.StdEncoding.EncodeToString(raw), strings.TrimSpace(in.Question))
}
