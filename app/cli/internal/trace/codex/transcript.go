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

package codex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

const (
	responseItem   = "response_item"
	customToolCall = "custom_tool_call"
	functionCall   = "function_call"
	messageItem    = "message"
)

// transcriptEntry normalizes messages and tool exchanges for every consumer,
// while retaining their original event and all unknown metadata records.
func transcriptEntry(e rolloutEntry, payload rolloutPayload, raw json.RawMessage, model string, line int) (json.RawMessage, error) {
	if e.Type != responseItem {
		return raw, nil
	}
	switch payload.Type {
	case messageItem, functionCall, customToolCall, functionCallOutput, customToolCallOutput, "reasoning":
	default:
		return raw, nil
	}
	var item struct {
		Content     json.RawMessage `json:"content"`
		Arguments   string          `json:"arguments"`
		Input       string          `json:"input"`
		Output      json.RawMessage `json:"output"`
		Replaced    json.RawMessage `json:"chainloop.replaced"`
		Path        string          `json:"path"`
		SpecSources json.RawMessage `json:"spec_sources"`
		Summary     []struct {
			Text string `json:"text"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(e.Payload, &item); err != nil {
		return nil, fmt.Errorf("read Codex transcript item on line %d: %w", line, err)
	}
	entry := trace.RawSessionEntry{
		UUID:        payload.ID,
		Timestamp:   e.Timestamp,
		SourceEvent: raw,
		Message:     trace.RawSessionMessage{Model: model},
	}
	if entry.UUID == "" {
		entry.UUID = fmt.Sprintf("codex-event-%d", line)
	}
	var blocks []any
	switch payload.Type {
	case messageItem:
		if payload.Role != trace.RoleUser && payload.Role != trace.RoleAssistant {
			return raw, nil
		}
		entry.Type = payload.Role
		content, err := transcriptContent(item.Content)
		if err != nil {
			return nil, fmt.Errorf("read Codex message on line %d: %w", line, err)
		}
		entry.Message.Content = content
	case functionCall, customToolCall:
		entry.Type = trace.RoleAssistant
		input := json.RawMessage(item.Arguments)
		var object map[string]json.RawMessage
		if payload.Type == customToolCall {
			if len(item.Replaced) > 0 {
				values := jsonObject{pointer.Marker: item.Replaced}
				if item.Path != "" {
					values["path"] = encodeJSON(item.Path)
				}
				if item.Input != "" {
					values["input"] = encodeJSON(item.Input)
				}
				if len(item.SpecSources) > 0 {
					values["spec_sources"] = item.SpecSources
				}
				input = encodeJSON(values)
			} else {
				input, _ = json.Marshal(map[string]string{"input": item.Input})
			}
		} else if json.Unmarshal(input, &object) != nil || object == nil {
			input, _ = json.Marshal(map[string]string{"arguments": item.Arguments})
		}
		blocks = append(blocks, trace.RawSessionToolUseBlock{
			Type: "tool_use", ID: payload.CallID, Name: payload.Name, Input: input,
		})
	case functionCallOutput, customToolCallOutput:
		entry.Type = trace.RoleUser
		output := item.Output
		if len(output) == 0 {
			output = json.RawMessage(`""`)
		}
		if bytes.HasPrefix(bytes.TrimSpace(output), []byte("[")) {
			var err error
			output, err = transcriptContent(output)
			if err != nil {
				return nil, fmt.Errorf("read Codex tool result on line %d: %w", line, err)
			}
		}
		blocks = append(blocks, trace.RawSessionToolResultBlock{
			Type: "tool_result", ToolUseID: payload.CallID, Content: output,
		})
	case "reasoning":
		var summary []string
		for _, part := range item.Summary {
			if part.Text != "" {
				summary = append(summary, part.Text)
			}
		}
		if len(summary) == 0 {
			return raw, nil
		}
		entry.Type = trace.RoleAssistant
		blocks = append(blocks, map[string]string{typeField: "thinking", "thinking": strings.Join(summary, "\n")})
	default:
		return raw, nil
	}
	entry.Message.Role = entry.Type
	if blocks != nil {
		entry.Message.Content, _ = json.Marshal(blocks)
	}
	return json.Marshal(entry)
}

func transcriptContent(content json.RawMessage) (json.RawMessage, error) {
	if len(content) == 0 || bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
		return json.RawMessage(`[]`), nil
	}
	var text string
	if json.Unmarshal(content, &text) == nil {
		return content, nil
	}
	var blocks []jsonObject
	if err := json.Unmarshal(content, &blocks); err != nil {
		return nil, err
	}
	for _, block := range blocks {
		switch jsonString(block[typeField]) {
		case "input_text", "output_text":
			block[typeField] = encodeJSON(textField)
		case blockInputImage:
			if replaced := block[pointer.Marker]; len(replaced) > 0 {
				block[typeField] = encodeJSON("image")
				block["source"] = replaced
				delete(block, pointer.Marker)
			} else if data, media, ok := decodeImageURL(jsonString(block["image_url"])); ok {
				block[typeField] = encodeJSON("image")
				block["source"] = encodeJSON(map[string]string{typeField: "base64", "media_type": media, "data": base64.StdEncoding.EncodeToString(data)})
			}
			if jsonString(block[typeField]) == "image" {
				delete(block, "image_url")
			}
		}
	}
	return json.Marshal(blocks)
}
