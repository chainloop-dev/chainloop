//nolint:revive // "trace" names the CLI feature; the runtime/trace clash is inert here
package trace

import "encoding/json"

// Attribution constants for file-level code attribution.
const (
	// AttributionAI marks file changes produced by an AI agent.
	AttributionAI = "ai"
	// AttributionHuman marks file changes produced by a human.
	AttributionHuman = "human"
)

// RawSessionEntry is the wire format for a single message in
// raw_session["main"]. All providers must emit entries matching this
// struct so the frontend's ConversationTimeline can render them
// uniformly. The shape mirrors Claude's JSONL transcript format, which
// the frontend was originally built against.
type RawSessionEntry struct {
	Type      string            `json:"type,omitempty"`
	Role      string            `json:"role,omitempty"`
	UUID      string            `json:"uuid,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
	IsMeta    bool              `json:"isMeta,omitempty"`
	Message   RawSessionMessage `json:"message"`
}

// RawSessionMessage is the message body inside a RawSessionEntry.
type RawSessionMessage struct {
	Role    string          `json:"role,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	Model   string          `json:"model,omitempty"`
}

// RawSessionTextBlock is a content block carrying plain text.
type RawSessionTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// RawSessionToolUseBlock is a content block representing a tool call.
type RawSessionToolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// MustRawSessionEntry marshals a RawSessionEntry to json.RawMessage.
// Panics only on impossible encoding failures (struct field bugs).
func MustRawSessionEntry(e RawSessionEntry) json.RawMessage {
	b, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return b
}
