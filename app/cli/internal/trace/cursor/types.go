package cursor

// cursorJSONLRecord matches a single line in a Cursor agent transcript.
// The transcript schema is minimal compared to Claude Code's: only role +
// content-blocks of type "text". No tool_use blocks, no token usage.
type cursorJSONLRecord struct {
	Role    string          `json:"role"`
	Message cursorJSONLBody `json:"message"`
}

type cursorJSONLBody struct {
	Content []cursorContentBlock `json:"content"`
}

type cursorContentBlock struct {
	// Type is the block kind. Observed values: "text" (plain message body)
	// and "tool_use" (assistant invoking a built-in tool such as ReadFile,
	// ApplyPatch, StrReplace, Grep, Glob).
	Type string `json:"type"`
	// Text holds the message body for "text" blocks.
	Text string `json:"text,omitempty"`
	// Name holds the tool name for "tool_use" blocks (empty otherwise).
	Name string `json:"name,omitempty"`
}
