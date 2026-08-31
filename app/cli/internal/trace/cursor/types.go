//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
