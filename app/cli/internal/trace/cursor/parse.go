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

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// parsedTranscript collects the per-session signals we extract from a
// Cursor JSONL transcript. Token usage stays empty because Cursor does not
// expose it in the transcript format.
type parsedTranscript struct {
	// UserMessages is the count of records with role "user".
	UserMessages int
	// AssistantMessages is the count of records with role "assistant".
	AssistantMessages int
	// ToolCounts maps tool name to invocation count, derived from
	// "tool_use" content blocks across assistant messages.
	ToolCounts map[string]int
	// RawRecords retains every JSONL line for evidence storage.
	RawRecords []json.RawMessage
}

// parseCursorJSONL reads a Cursor agent transcript JSONL file and aggregates
// message and tool-use counts.
func parseCursorJSONL(path string) (*parsedTranscript, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	// Transcripts can contain long lines (file contents embedded in a prompt, etc.).
	// Match Claude parser's generous buffer to avoid tokenizer errors on wide records.
	const maxLineBytes = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	out := &parsedTranscript{ToolCounts: map[string]int{}}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec cursorJSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Malformed line (e.g. a truncated final record while Cursor is
			// still writing). Skip it: storing invalid JSON as a RawMessage
			// would fail the evidence encoder and abort the attestation.
			continue
		}

		out.RawRecords = append(out.RawRecords, json.RawMessage(bytes.Clone(line)))

		switch rec.Role {
		case "user":
			out.UserMessages++
		case "assistant":
			out.AssistantMessages++
		}

		for _, block := range rec.Message.Content {
			if block.Type == "tool_use" && block.Name != "" {
				out.ToolCounts[block.Name]++
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil && !errors.Is(scanErr, io.EOF) {
		return out, fmt.Errorf("scan cursor transcript: %w", scanErr)
	}

	return out, nil
}
