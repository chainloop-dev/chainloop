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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

const (
	nativeTurnContext    = "turn_context"
	nativeEventMsg       = "event_msg"
	contentField         = "content"
	callIDField          = "call_id"
	cwdField             = "cwd"
	textField            = "text"
	nativeSessionMeta    = "session_meta"
	mainTranscriptKey    = "main"
	functionCallOutput   = "function_call_output"
	customToolCallOutput = "custom_tool_call_output"
	blockInputImage      = "input_image"
	toolExecCommand      = "exec_command"
	toolApplyPatch       = "apply_patch"
	typeField            = "type"
	outputField          = "output"
	valueField           = "value"
)

var catPathPattern = regexp.MustCompile(`\bcat\s+(?:--\s+)?(?:"([^"\n]+)"|'([^'\n]+)'|([^\s"';&|()]+))`)

var codeStringPattern = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

type sourceCopies map[string]string

// commandSources loads only explicitly read files, with the same text-size
// bound as local spec capture. Their full bytes must match a stored source.
func commandSources(input, cwd string, sources pointer.Sources) sourceCopies {
	copies := sourceCopies{}
	for _, match := range catPathPattern.FindAllStringSubmatch(input, -1) {
		path := ""
		for _, candidate := range match[1:] {
			if candidate != "" {
				path = candidate
			}
		}
		if !filepath.IsAbs(path) {
			if !filepath.IsAbs(cwd) {
				continue
			}
			path = filepath.Join(cwd, path)
		}
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			_ = f.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		_ = f.Close()
		if err != nil || len(data) == 0 || len(data) > 1<<20 {
			continue
		}
		if material, ok := sources[pointer.Digest(data)]; ok {
			copies[string(data)] = material
		}
	}
	return copies
}

func toolCommand(fields jsonObject) (string, string) {
	if input := jsonString(fields["input"]); input != "" {
		return input, ""
	}
	args := object(json.RawMessage(jsonString(fields["arguments"])))
	cmd := jsonString(args["cmd"])
	if cmd == "" {
		cmd = jsonString(args["command"])
	}
	return cmd, jsonString(args["workdir"])
}

var (
	_ trace.SpecCopyReplacer  = (*Provider)(nil)
	_ trace.PastedImageFinder = (*Provider)(nil)
)

// jsonObject keeps unmodified and future fields without decoding them into any.
type jsonObject map[string]json.RawMessage

func object(raw json.RawMessage) jsonObject {
	var v jsonObject
	_ = json.Unmarshal(raw, &v)
	return v
}

func jsonString(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

func encodeJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v) // all callers supply JSON values or typed structures
	return b
}

// nativeRecord selects exactly one representation of an event. Normalized
// messages also retain the event, which must not count as another use or paste.
func nativeRecord(raw json.RawMessage) (rolloutEntry, rolloutPayload, bool) {
	var wrapper struct {
		SourceEvent json.RawMessage `json:"source_event"`
	}
	_ = json.Unmarshal(raw, &wrapper)
	if len(wrapper.SourceEvent) > 0 {
		raw = wrapper.SourceEvent
	}
	var e rolloutEntry
	var payload rolloutPayload
	ok := json.Unmarshal(raw, &e) == nil && e.Type != "" && json.Unmarshal(e.Payload, &payload) == nil
	return e, payload, ok
}

// updateNative preserves wrapper metadata and rebuilds only viewer content
// from the replaced native payload. Neither representation keeps an old copy.
func updateNative(raw json.RawMessage, payload jsonObject) json.RawMessage {
	fields := object(raw)
	if fields == nil {
		return raw
	}
	if source := fields["source_event"]; len(source) > 0 {
		native := object(source)
		if native == nil {
			return raw
		}
		native["payload"] = encodeJSON(payload)
		updated := encodeJSON(native)
		e, p, ok := nativeRecord(updated)
		if !ok {
			return raw
		}
		var wrapper trace.RawSessionEntry
		if json.Unmarshal(raw, &wrapper) != nil {
			return raw
		}
		converted, err := transcriptEntry(e, p, updated, wrapper.Message.Model, 0)
		if err != nil {
			return raw
		}
		message := object(converted)["message"]
		if len(message) == 0 {
			return raw
		}
		fields["message"] = message
		fields["source_event"] = updated
	} else {
		fields["payload"] = encodeJSON(payload)
	}
	return encodeJSON(fields)
}

func orderedStreams(raw map[string][]json.RawMessage) []string {
	var keys []string
	for key := range raw {
		if key != mainTranscriptKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if _, ok := raw[mainTranscriptKey]; ok {
		keys = append([]string{mainTranscriptKey}, keys...)
	}
	return keys
}

func decodeImageURL(url string) ([]byte, string, bool) {
	header, encoded, ok := strings.Cut(url, ",")
	if !ok || !strings.HasPrefix(header, "data:image/") || !strings.HasSuffix(header, ";base64") {
		return nil, "", false
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	return data, strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"), err == nil && len(data) > 0
}

// promptImages visits user input only. Tool images are deliberately excluded.
func promptImages(e rolloutEntry, p rolloutPayload) ([]jsonObject, jsonObject) {
	fields := object(e.Payload)
	if fields == nil {
		return nil, nil
	}
	if e.Type == responseItem && p.Type == messageItem && p.Role == trace.RoleUser {
		var blocks []jsonObject
		if json.Unmarshal(fields[contentField], &blocks) != nil {
			return nil, nil
		}
		return blocks, fields
	}
	return nil, nil
}

// PastedImages extracts each distinct user image from native or normalized
// rollouts. The digest identifies the model-visible bytes, not a file path.
func (p *Provider) PastedImages(raw map[string][]json.RawMessage) []trace.PastedImage {
	var images []trace.PastedImage
	seen := map[string]bool{}
	position := 0
	for _, key := range orderedStreams(raw) {
		for _, line := range raw[key] {
			e, payload, ok := nativeRecord(line)
			if !ok {
				continue
			}
			blocks, _ := promptImages(e, payload)
			for _, b := range blocks {
				if jsonString(b[typeField]) != blockInputImage {
					continue
				}
				position++
				data, media, valid := decodeImageURL(jsonString(b["image_url"]))
				if !valid {
					continue
				}
				digest := pointer.Digest(data)
				if !seen[digest] {
					seen[digest] = true
					images = append(images, trace.PastedImage{Data: data, Digest: digest, MediaType: media, Number: position, Timestamp: eventTimestamp(e.Timestamp)})
				}
			}
		}
	}
	return images
}

func eventTimestamp(timestamp string) string {
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// ReplaceSpecCopies replaces exact source copies only after material storage.
// Results and native duplicates use the same digest mapping as viewer messages.
func (p *Provider) ReplaceSpecCopies(raw map[string][]json.RawMessage, sources pointer.Sources) pointer.Report {
	report := pointer.Report{}
	if len(sources) == 0 {
		return report
	}
	for _, lines := range raw {
		calls := map[string]rolloutPayload{}
		results := map[string]json.RawMessage{}
		copies := sourceCopies{}
		cwd := ""
		for _, line := range lines {
			e, payload, ok := nativeRecord(line)
			if !ok {
				continue
			}
			fields := object(e.Payload)
			if e.Type == nativeSessionMeta || e.Type == nativeTurnContext {
				if dir := jsonString(fields[cwdField]); dir != "" {
					cwd = dir
				}
			}
			if e.Type == nativeEventMsg && payload.Type == "item_completed" {
				item := object(fields["item"])
				if jsonString(item[typeField]) == "CommandExecution" {
					addSourceCopies(copies, commandSources(jsonString(item["command"]), jsonString(item[cwdField]), sources))
				}
			}
			if e.Type != responseItem {
				continue
			}
			switch payload.Type {
			case functionCall, customToolCall:
				calls[payload.CallID] = payload
				cmd, workdir := toolCommand(fields)
				if workdir == "" {
					workdir = cwd
				}
				addSourceCopies(copies, commandSources(cmd, workdir, sources))
			case functionCallOutput, customToolCallOutput:
				results[payload.CallID] = object(e.Payload)[outputField]
			}
		}
		for i, line := range lines {
			e, payload, ok := nativeRecord(line)
			if !ok {
				continue
			}
			fields := object(e.Payload)
			changed := replacePromptImages(e, payload, fields, sources, report)
			if e.Type == nativeEventMsg && payload.Type == "item_completed" {
				changed = replaceTimelineItem(fields, sources, copies, report) || changed
			}
			if e.Type == responseItem {
				switch payload.Type {
				case functionCallOutput, customToolCallOutput:
					call := calls[payload.CallID]
					if isReadTool(call.Name) {
						replacer := outputReplacer{sources: sources, copies: copies}
						if value, n := replacer.replace(fields[outputField]); n > 0 {
							fields[outputField] = value
							if len(replacer.metadata) > 0 {
								fields["codex_output"] = encodeJSON(replacer.metadata)
							}
							res := report.Finder(pointer.FinderFileRead)
							res.Replaced += n
							changed = true
						}
					}
				case functionCall, customToolCall:
					if result, found := results[payload.CallID]; found && !failedOutput(result) {
						changed = replaceWrite(fields, payload.Name, sources, report) || changed
					}
				}
			}
			if changed {
				lines[i] = updateNative(line, fields)
			}
		}
	}
	return report
}

func addSourceCopies(dst, src sourceCopies) {
	for text, material := range src {
		dst[text] = material
	}
}

func replaceTimelineItem(fields jsonObject, sources pointer.Sources, copies sourceCopies, report pointer.Report) bool {
	item := object(fields["item"])
	changed := false
	switch jsonString(item[typeField]) {
	case "CommandExecution":
		if failedOutput(encodeJSON(item)) {
			return false
		}
		if updated, n := replaceOutput(item["aggregated_output"], sources, copies); n > 0 {
			storeTimelineCopy(item, "aggregated_output", updated)
			report.Finder(pointer.FinderFileRead).Replaced += n
			changed = true
		}
	case "FileChange":
		if jsonString(item["status"]) == "failed" {
			return false
		}
		changes := object(item["changes"])
		for path, raw := range changes {
			change := object(raw)
			if jsonString(change[typeField]) != "add" {
				continue
			}
			if updated, n := replaceOutput(change[contentField], sources, copies); n > 0 {
				storeTimelineCopy(change, contentField, updated)
				changes[path] = encodeJSON(change)
				report.Finder(pointer.FinderFileWrite).Replaced += n
				changed = true
			}
		}
		if changed {
			item["changes"] = encodeJSON(changes)
		}
	}
	if changed {
		fields["item"] = encodeJSON(item)
	}
	return changed
}

func storeTimelineCopy(fields jsonObject, original string, updated json.RawMessage) {
	delete(fields, original)
	var blocks []json.RawMessage
	if json.Unmarshal(updated, &blocks) == nil && len(blocks) == 1 && jsonString(object(blocks[0])[typeField]) == pointer.Marker {
		fields[pointer.Marker] = blocks[0]
		return
	}
	fields[contentField] = updated
}

func isReadTool(name string) bool {
	switch name {
	case "exec", toolExecCommand, "shell", "shell_command", "Bash", "read_file", "read", "write_stdin":
		return true
	}
	return false
}

func replacePromptImages(e rolloutEntry, payload rolloutPayload, fields jsonObject, sources pointer.Sources, report pointer.Report) bool {
	blocks, _ := promptImages(e, payload)
	changed := false
	for _, b := range blocks {
		if jsonString(b[typeField]) != blockInputImage || len(b[pointer.Marker]) > 0 {
			continue
		}
		data, _, ok := decodeImageURL(jsonString(b["image_url"]))
		res := report.Finder(pointer.FinderPastedImage)
		if !ok {
			res.Skip(pointer.ReasonInvalidImage, 1)
			continue
		}
		material, stored := sources[pointer.Digest(data)]
		if !stored {
			res.Skip(pointer.ReasonMaterialNotStored, 1)
			continue
		}
		res.BytesRemoved += len(b["image_url"])
		delete(b, "image_url")
		b[pointer.Marker] = pointer.New(material)
		res.Replaced++
		changed = true
	}
	if changed {
		fields[contentField] = encodeJSON(blocks)
	}
	return changed
}

// stdoutText strips only the documented shell result envelope. It never
// trims the source text: trailing newlines participate in its digest.
func stdoutText(text string) (string, bool) {
	if strings.Contains(text, "Process exited with code ") {
		prefix, output, ok := strings.Cut(text, "\nFinal output:\n")
		if !ok || !strings.HasSuffix(prefix, "Process exited with code 0") {
			return "", false
		}
		return output, true
	}
	return text, true
}

func failedOutput(raw json.RawMessage) bool {
	text := jsonString(raw)
	if text != "" {
		for _, prefix := range []string{"Error:", "error:", "Failed to ", "apply_patch verification failed:", "Script failed"} {
			if strings.HasPrefix(strings.TrimSpace(text), prefix) {
				return true
			}
		}
		_, ok := stdoutText(text)
		return !ok
	}
	var blocks []struct{ Text string }
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if failedOutput(encodeJSON(block.Text)) {
				return true
			}
		}
	}
	var result struct {
		ExitCode *int `json:"exit_code"`
		IsError  bool `json:"is_error"`
	}
	return json.Unmarshal(raw, &result) == nil && (result.IsError || result.ExitCode != nil && *result.ExitCode != 0)
}

type outputReplacer struct {
	sources  pointer.Sources
	copies   sourceCopies
	metadata []json.RawMessage
}

func replaceOutput(raw json.RawMessage, sources pointer.Sources, copies sourceCopies) (json.RawMessage, int) {
	r := outputReplacer{sources: sources, copies: copies}
	return r.replace(raw)
}

func (r *outputReplacer) replace(raw json.RawMessage) (json.RawMessage, int) {
	// Structured failure status wins over source matching. A text prefix is
	// only a heuristic: a source document can itself start with "Error:".
	if object(raw) != nil && failedOutput(raw) {
		return raw, 0
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		stdout, ok := stdoutText(text)
		if ok {
			if material, found := r.sources[pointer.Digest([]byte(stdout))]; found {
				return encodeJSON([]json.RawMessage{pointer.New(material)}), 1
			}
		}
		if failedOutput(raw) {
			return raw, 0
		}
		if ok {
			if updated, n := replaceTextCopies(stdout, r.copies); n > 0 {
				return updated, n
			}
		}
		// Code mode exposes nested exec_command results as JSON text blocks.
		if nested := object(json.RawMessage(text)); nested != nil {
			if updated, n := r.replace(json.RawMessage(text)); n > 0 {
				// Metadata belongs to the native event, outside canonical pointers.
				if blocks := outputContent(updated); len(blocks) > 0 {
					r.metadata = append(r.metadata, updated)
					return encodeJSON(blocks), n
				}
			}
		}
		return raw, 0
	}
	if fields := object(raw); fields != nil {
		for _, key := range []string{outputField, valueField} {
			if value, n := r.replace(fields[key]); n > 0 {
				fields[key] = value
				return encodeJSON(fields), n
			}
		}
		return raw, 0
	}
	var blocks []jsonObject
	if json.Unmarshal(raw, &blocks) != nil {
		return raw, 0
	}
	if len(blocks) > 0 {
		frame := jsonString(blocks[0][textField])
		if strings.HasPrefix(frame, "Script failed\n") && strings.Contains(frame, "\nWall time") {
			return raw, 0
		}
	}
	n := 0
	var replaced []jsonObject
	for _, block := range blocks {
		if kind := jsonString(block[typeField]); kind != "input_text" && kind != "output_text" && kind != textField {
			replaced = append(replaced, block)
			continue
		}
		if updated, count := r.replace(block[textField]); count > 0 {
			var values []jsonObject
			if json.Unmarshal(updated, &values) == nil && len(values) > 0 {
				replaced = append(replaced, values...)
				n += count
				continue
			}
		}
		replaced = append(replaced, block)
	}
	if n > 0 {
		return encodeJSON(replaced), n
	}
	return raw, 0
}

// outputContent finds the replaced result blocks inside a recognized envelope.
// It preserves surrounding text in addition to material pointers.
func outputContent(raw json.RawMessage) []json.RawMessage {
	if fields := object(raw); fields != nil {
		for _, key := range []string{outputField, valueField} {
			if blocks := outputContent(fields[key]); len(blocks) > 0 {
				return blocks
			}
		}
		return nil
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		return blocks
	}
	return nil
}

func replaceTextCopies(text string, copies sourceCopies) (json.RawMessage, int) {
	// Longest first makes overlapping sources deterministic.
	keys := make([]string, 0, len(copies))
	for source := range copies {
		keys = append(keys, source)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	var blocks []json.RawMessage
	n := 0
	for text != "" {
		position, matched := -1, ""
		for _, source := range keys {
			index := sourceOffset(text, source)
			if index >= 0 && (position < 0 || index < position) {
				position, matched = index, source
			}
		}
		if position < 0 {
			blocks = append(blocks, encodeJSON(trace.RawSessionTextBlock{Type: textField, Text: text}))
			break
		}
		if position > 0 {
			blocks = append(blocks, encodeJSON(trace.RawSessionTextBlock{Type: textField, Text: text[:position]}))
		}
		blocks = append(blocks, pointer.New(copies[matched]))
		text = text[position+len(matched):]
		n++
	}
	if n == 0 {
		return nil, 0
	}
	return encodeJSON(blocks), n
}

// sourceOffset requires complete source boundaries, so a source that merely
// appears inside an unrelated line is not treated as a file read.
func sourceOffset(text, source string) int {
	if source == "" {
		return -1
	}
	start := 0
	for start < len(text) {
		rel := strings.Index(text[start:], source)
		if rel < 0 {
			return -1
		}
		index := start + rel
		end := index + len(source)
		if (index == 0 || text[index-1] == '\n') && (end == len(text) || strings.HasSuffix(source, "\n") || text[end] == '\n') {
			return index
		}
		start = index + 1
	}
	return -1
}

func replaceWrite(fields jsonObject, name string, sources pointer.Sources, report pointer.Report) bool {
	if name == "exec" {
		return replaceCodePatch(fields, sources, report)
	}
	if name == toolApplyPatch {
		patch := jsonString(fields["input"])
		// A complete file creation has exact source bytes. Updates and mixed
		// patches stay inline rather than guessing their resulting content.
		lines := strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
		if len(lines) < 4 || lines[0] != "*** Begin Patch" || !strings.HasPrefix(lines[1], "*** Add File: ") || lines[len(lines)-1] != "*** End Patch" {
			return false
		}
		var content strings.Builder
		for _, line := range lines[2 : len(lines)-1] {
			if !strings.HasPrefix(line, "+") {
				return false
			}
			content.WriteString(strings.TrimPrefix(line, "+"))
			content.WriteByte('\n')
		}
		if material, ok := sources[pointer.Digest([]byte(content.String()))]; ok {
			delete(fields, "input")
			fields[pointer.Marker] = pointer.New(material)
			fields["path"] = encodeJSON(strings.TrimPrefix(lines[1], "*** Add File: "))
			report.Finder(pointer.FinderFileWrite).Replaced++
			return true
		}
		return false
	}
	if name != "write_file" && name != "write" {
		return false
	}
	args := object(json.RawMessage(jsonString(fields["arguments"])))
	if args == nil || len(args[contentField]) == 0 {
		return false
	}
	material, ok := sources[pointer.Digest([]byte(jsonString(args[contentField])))]
	if !ok {
		return false
	}
	delete(args, contentField)
	args[pointer.Marker] = pointer.New(material)
	fields["arguments"] = encodeJSON(string(encodeJSON(args)))
	report.Finder(pointer.FinderFileWrite).Replaced++
	return true
}

// replaceCodePatch recognizes literal patches in Codex code mode. It never
// evaluates JavaScript or reconstructs a computed patch. Other script text
// stays available alongside the material pointer in the viewer.
func replaceCodePatch(fields jsonObject, sources pointer.Sources, report pointer.Report) bool {
	code := jsonString(fields["input"])
	if !strings.Contains(code, "tools.apply_patch") {
		return false
	}
	var out strings.Builder
	position := 0
	var replacements []json.RawMessage
	for _, span := range codeStringPattern.FindAllStringIndex(code, -1) {
		literal := jsonString(json.RawMessage(code[span[0]:span[1]]))
		if !strings.HasPrefix(literal, "*** Begin Patch\n") {
			continue
		}
		patch := jsonObject{"input": encodeJSON(literal)}
		if !replaceWrite(patch, toolApplyPatch, sources, report) {
			continue
		}
		out.WriteString(code[position:span[0]])
		out.Write(patch[pointer.Marker])
		position = span[1]
		replacements = append(replacements, patch[pointer.Marker])
	}
	if len(replacements) == 0 {
		return false
	}
	out.WriteString(code[position:])
	fields["input"] = encodeJSON(out.String())
	fields[pointer.Marker] = replacements[0]
	if len(replacements) > 1 {
		fields["spec_sources"] = encodeJSON(replacements)
	}
	return true
}

// outputTexts reads successful text results, including code-mode envelopes,
// for skill-load recognition without interpreting arbitrary tool arguments.
func outputTexts(raw json.RawMessage) []string {
	if failedOutput(raw) {
		return nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if fields := object(json.RawMessage(text)); fields != nil && (len(fields[outputField]) > 0 || len(fields[valueField]) > 0) {
			return outputTexts(json.RawMessage(text))
		}
		stdout, ok := stdoutText(text)
		if ok {
			return []string{stdout}
		}
		return nil
	}
	if fields := object(raw); fields != nil {
		for _, key := range []string{outputField, valueField} {
			if value, ok := fields[key]; ok {
				return outputTexts(value)
			}
		}
		return nil
	}
	var blocks []jsonObject
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	var texts []string
	for _, block := range blocks {
		if bytes.Contains(block[typeField], []byte(textField)) {
			texts = append(texts, outputTexts(block[textField])...)
		}
	}
	return texts
}
