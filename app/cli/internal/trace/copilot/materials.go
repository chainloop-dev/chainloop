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

package copilot

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
)

var (
	_ trace.SpecCopyReplacer  = (*Provider)(nil)
	_ trace.PastedImageFinder = (*Provider)(nil)
)

const (
	// viewTool reads a file, or a range of its lines.
	viewTool = "view"
	// createTool writes a new file with the text of its file_text argument.
	createTool = "create"

	sourceEventField = "source_event"
	dataField        = "data"
	resultField      = "result"
	// textBlock is the type of a text block of a viewer message.
	textBlock = "text"
)

// shellEnvelope matches the status line that Copilot adds after the output
// of a shell tool: "<exited with exit code N>" in older releases, and
// "<shellId: ID completed with exit code N>" in newer ones.
var shellEnvelope = regexp.MustCompile(`(?s)^(.*?)\n?(<(?:exited|shellId: [^>\n]* completed) with exit code (-?\d+)>\s*)$`)

// jsonObject keeps the fields of an event that Trace does not change, and
// fields that a later Copilot release adds, without a decode into any.
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
	b, _ := json.Marshal(v) // all callers give JSON values or typed structures
	return b
}

// nativeEvent returns the event of a raw session line as Copilot wrote it:
// the source_event of a viewer line, or the line itself.
func nativeEvent(line json.RawMessage) (logEvent, jsonObject, bool) {
	raw := line
	if source := object(line)[sourceEventField]; len(source) > 0 {
		raw = source
	}
	var e logEvent
	if json.Unmarshal(raw, &e) != nil || e.Type == "" {
		return logEvent{}, nil, false
	}
	data := object(e.Data)
	return e, data, data != nil
}

// updateEvent writes data into the event of a raw session line. For a viewer
// line, it also builds the viewer message again from the changed event, so
// that no copy stays in either form.
func updateEvent(line json.RawMessage, data jsonObject) json.RawMessage {
	fields := object(line)
	if fields == nil {
		return line
	}
	source, wrapped := fields[sourceEventField]
	native := fields
	if wrapped {
		if native = object(source); native == nil {
			return line
		}
	}
	native[dataField] = encodeJSON(data)
	updated := encodeJSON(native)
	if !wrapped {
		return updated
	}

	var e logEvent
	var d eventData
	if json.Unmarshal(updated, &e) != nil || json.Unmarshal(e.Data, &d) != nil {
		return line
	}
	rebuilt := object(rawEntry(e, d, updated, 0))
	if rebuilt == nil {
		return line
	}
	// An event with no ID has an ID from its line number: keep it.
	if uuid, ok := fields["uuid"]; ok {
		rebuilt["uuid"] = uuid
	}
	return encodeJSON(rebuilt)
}

// orderedStreams returns the keys of the raw session: main first, then the
// other streams by name.
func orderedStreams(raw map[string][]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key != mainLogKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if _, ok := raw[mainLogKey]; ok {
		keys = append([]string{mainLogKey}, keys...)
	}
	return keys
}

// binaryAsset is a session.binary_asset event: Copilot keeps each image that
// the model gets one time, by its digest, and refers to it by assetId.
type binaryAsset struct {
	AssetID  string `json:"assetId"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// attachment is a file that the user attached to a message. A pasted or
// attached image has an image media type and an assetId, or its data.
type attachment struct {
	AssetID  string          `json:"assetId"`
	MimeType string          `json:"mimeType"`
	Data     string          `json:"data"`
	Replaced json.RawMessage `json:"chainloop.replaced"`
}

func (a attachment) isImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}

// decodeImage decodes the base64 data of an image. When the image has an
// asset ID, the data must have that digest.
func decodeImage(encoded, assetID string) ([]byte, bool) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	if assetID != "" && pointer.Digest(data) != assetID {
		return nil, false
	}
	return data, true
}

// userAttachments returns the attachments of a message of the user. A
// message of a subagent is the prompt of the parent agent, not of the user.
func userAttachments(e logEvent, data jsonObject) []attachment {
	if e.Type != eventUserMessage || e.AgentID != "" {
		return nil
	}
	var attachments []attachment
	_ = json.Unmarshal(data["attachments"], &attachments)
	return attachments
}

// streamAssets returns the image assets of a stream that still hold their
// data, by asset ID.
func streamAssets(lines []json.RawMessage) map[string]binaryAsset {
	assets := map[string]binaryAsset{}
	for _, line := range lines {
		e, _, ok := nativeEvent(line)
		if !ok || e.Type != eventBinaryAsset {
			continue
		}
		var a binaryAsset
		if json.Unmarshal(e.Data, &a) == nil && a.AssetID != "" && a.Data != "" {
			assets[a.AssetID] = a
		}
	}
	return assets
}

// PastedImages returns each distinct image that the user attached to a
// message. An image that a tool gave the model, such as a view of an image
// file, is not a pasted image (spec issue-3800 R-009).
func (p *Provider) PastedImages(raw map[string][]json.RawMessage) []trace.PastedImage {
	var images []trace.PastedImage
	seen := map[string]bool{}
	position := 0
	for _, key := range orderedStreams(raw) {
		lines := raw[key]
		assets := streamAssets(lines)
		for _, line := range lines {
			e, data, ok := nativeEvent(line)
			if !ok {
				continue
			}
			for _, a := range userAttachments(e, data) {
				if !a.isImage() {
					continue
				}
				position++
				encoded := a.Data
				if encoded == "" {
					encoded = assets[a.AssetID].Data
				}
				img, valid := decodeImage(encoded, a.AssetID)
				if !valid {
					continue
				}
				digest := pointer.Digest(img)
				if seen[digest] {
					continue
				}
				seen[digest] = true
				images = append(images, trace.PastedImage{
					Data: img, Digest: digest, MediaType: a.MimeType, Number: position, Timestamp: eventTimestamp(e.Timestamp),
				})
			}
		}
	}
	return images
}

// streamCalls is what ReplaceSpecCopies learns from a stream before it
// changes it.
type streamCalls struct {
	// tools holds the tool name of each tool call ID.
	tools map[string]string
	// succeeded holds the tool call IDs that completed with success.
	succeeded map[string]bool
	// pasted holds the asset IDs of the images that the user attached.
	pasted map[string]bool
	// args holds the arguments of each tool call ID.
	args map[string]json.RawMessage
	// cwd is the working folder of the session.
	cwd string
	// copies holds, for each shell tool call ID, the sources that the
	// command read with cat. Only the output of that call can hold them.
	copies map[string]sourceCopies
}

func readStreamCalls(lines []json.RawMessage) streamCalls {
	s := streamCalls{tools: map[string]string{}, succeeded: map[string]bool{}, pasted: map[string]bool{}, args: map[string]json.RawMessage{}}
	for _, line := range lines {
		e, data, ok := nativeEvent(line)
		if !ok {
			continue
		}
		switch e.Type {
		case eventSessionStartRecord:
			var start struct {
				Context struct {
					Cwd string `json:"cwd"`
				} `json:"context"`
			}
			if json.Unmarshal(e.Data, &start) == nil && start.Context.Cwd != "" {
				s.cwd = start.Context.Cwd
			}
		case eventAssistantMessage:
			var requests []struct {
				ToolCallID string          `json:"toolCallId"`
				Name       string          `json:"name"`
				Arguments  json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(data["toolRequests"], &requests)
			for _, r := range requests {
				if r.ToolCallID != "" {
					s.tools[r.ToolCallID] = r.Name
					s.args[r.ToolCallID] = r.Arguments
				}
			}
		case eventToolComplete:
			if completed(data) {
				s.succeeded[jsonString(data["toolCallId"])] = true
			}
		case eventUserMessage:
			for _, a := range userAttachments(e, data) {
				if a.isImage() && a.AssetID != "" {
					s.pasted[a.AssetID] = true
				}
			}
		}
	}
	return s
}

// completed reports whether a tool call completed with success. A shell
// command with an exit code other than 0 has success true, so the exit code
// is checked too.
func completed(data jsonObject) bool {
	var status struct {
		Success        bool `json:"success"`
		ShellExecution *struct {
			ExitCode *int `json:"exitCode"`
		} `json:"shellExecution"`
	}
	if json.Unmarshal(encodeJSON(data), &status) != nil || !status.Success {
		return false
	}
	return status.ShellExecution == nil || status.ShellExecution.ExitCode == nil || *status.ShellExecution.ExitCode == 0
}

// ReplaceSpecCopies replaces each exact copy of a stored source with a pointer
// to its material: the full view or shell read of a source file, the text of
// a create call or of a patch that adds one file, and the data of an image
// that the user attached. Partial reads, changed content, failed tool calls,
// and data that cannot be decoded stay inline (spec issue-3800 R-009).
func (p *Provider) ReplaceSpecCopies(raw map[string][]json.RawMessage, sources pointer.Sources) pointer.Report {
	report := pointer.Report{}
	if len(sources) == 0 {
		return report
	}
	for _, lines := range raw {
		calls := readStreamCalls(lines)
		calls.copies = commandSources(calls, sources)
		images := storedImages(lines, calls.pasted, sources, report)
		for i, line := range lines {
			e, data, ok := nativeEvent(line)
			if !ok {
				continue
			}
			changed := false
			switch e.Type {
			case eventBinaryAsset:
				changed = replaceAsset(data, images, report)
			case eventUserMessage:
				changed = replaceAttachments(e, data, images, sources, report)
			case eventToolComplete:
				changed = replaceRead(data, calls, sources, report)
			case eventAssistantMessage:
				changed = replaceWrites(data, calls, sources, report)
			case eventHookStart:
				changed = replaceHookInput(data, calls, sources, report)
			}
			if changed {
				lines[i] = updateEvent(line, data)
			}
		}
	}
	return report
}

// storedImages returns the material of each pasted image asset that is
// stored, by asset ID. It counts the assets that stay inline.
func storedImages(lines []json.RawMessage, pasted map[string]bool, sources pointer.Sources, report pointer.Report) map[string]string {
	images := map[string]string{}
	for id, asset := range streamAssets(lines) {
		if !pasted[id] {
			continue
		}
		res := report.Finder(pointer.FinderPastedImage)
		if _, ok := decodeImage(asset.Data, id); !ok {
			res.Skip(pointer.ReasonInvalidImage, 1)
			continue
		}
		material, ok := sources[id]
		if !ok {
			res.Skip(pointer.ReasonMaterialNotStored, 1)
			continue
		}
		images[id] = material
	}
	return images
}

// replaceAsset replaces the data of a stored pasted image with a pointer.
func replaceAsset(data jsonObject, images map[string]string, report pointer.Report) bool {
	material, ok := images[jsonString(data["assetId"])]
	if !ok || len(data[dataField]) == 0 {
		return false
	}
	res := report.Finder(pointer.FinderPastedImage)
	res.Replaced++
	res.BytesRemoved += len(data[dataField])
	delete(data, dataField)
	data[pointer.Marker] = pointer.New(material)
	return true
}

// replaceAttachments marks each stored image attachment with its pointer, so
// that the viewer shows it, and replaces the data of an attachment that holds
// its image.
func replaceAttachments(e logEvent, data jsonObject, images map[string]string, sources pointer.Sources, report pointer.Report) bool {
	var attachments []jsonObject
	if userAttachments(e, data) == nil || json.Unmarshal(data["attachments"], &attachments) != nil {
		return false
	}
	changed := false
	for _, fields := range attachments {
		var a attachment
		if json.Unmarshal(encodeJSON(fields), &a) != nil || !a.isImage() || len(a.Replaced) > 0 {
			continue
		}
		if a.Data != "" {
			res := report.Finder(pointer.FinderPastedImage)
			img, ok := decodeImage(a.Data, a.AssetID)
			if !ok {
				res.Skip(pointer.ReasonInvalidImage, 1)
				continue
			}
			material, ok := sources[pointer.Digest(img)]
			if !ok {
				res.Skip(pointer.ReasonMaterialNotStored, 1)
				continue
			}
			res.Replaced++
			res.BytesRemoved += len(fields[dataField])
			delete(fields, dataField)
			fields[pointer.Marker] = pointer.New(material)
			changed = true
			continue
		}
		if material, ok := images[a.AssetID]; ok {
			fields[pointer.Marker] = pointer.New(material)
			changed = true
		}
	}
	if changed {
		data["attachments"] = encodeJSON(attachments)
	}
	return changed
}

// splitShellResult returns the output of a successful shell command with the
// newline before the status line dropped, and the offset of the status line.
// It returns false when the command failed or the status is not known.
// Copilot drops the last newline of the output, so the caller tries the
// output with and without it.
func splitShellResult(content string) (string, int, bool) {
	m := shellEnvelope.FindStringSubmatchIndex(content)
	if m == nil || content[m[6]:m[7]] != "0" {
		return "", 0, false
	}
	return content[m[2]:m[3]], m[4], true
}

// readReplacement returns what replaces the result text of a tool call that
// holds copies of sources, and the number of copies. A full view of a source,
// or a shell output that is exactly a source, becomes one pointer. A shell
// output that holds sources that the same command read with cat becomes text
// and pointer blocks. A view with a line range is a partial read, so it stays
// inline, also when the range covers the whole file.
func readReplacement(tool string, args json.RawMessage, copies sourceCopies, content string, sources pointer.Sources) (json.RawMessage, int) {
	switch {
	case tool == viewTool:
		if _, ranged := object(args)["view_range"]; ranged {
			return nil, 0
		}
		if material, ok := sources[pointer.Digest([]byte(content))]; content != "" && ok {
			return pointer.New(material), 1
		}
	case slices.Contains(commandTools, tool):
		output, status, ok := splitShellResult(content)
		if !ok {
			return nil, 0
		}
		for _, candidate := range []string{output, output + "\n"} {
			if material, ok := sources[pointer.Digest([]byte(candidate))]; candidate != "" && ok {
				return pointer.New(material), 1
			}
		}
		// The text before the status line keeps its last newline, so that a
		// source at the end of the output ends at a line boundary. The status
		// line stays after the blocks of the output.
		if blocks, n := replaceTextCopies(content[:status], copies); n > 0 {
			blocks = append(blocks, encodeJSON(trace.RawSessionTextBlock{Type: textBlock, Text: content[status:]}))
			return encodeJSON(blocks), n
		}
	}
	return nil, 0
}

// replaceRead replaces the copies of sources in the result of a read.
func replaceRead(data jsonObject, calls streamCalls, sources pointer.Sources, report pointer.Report) bool {
	id := jsonString(data["toolCallId"])
	result := object(data[resultField])
	if result == nil || len(result[pointer.Marker]) > 0 || !calls.succeeded[id] {
		return false
	}
	replacement, n := readReplacement(calls.tools[id], calls.args[id], calls.copies[id], jsonString(result["content"]), sources)
	if n == 0 {
		return false
	}
	res := report.Finder(pointer.FinderFileRead)
	res.Replaced += n
	res.BytesRemoved += len(result["content"]) + len(result["detailedContent"])
	// detailedContent is the same read, shown as a diff.
	delete(result, "content")
	delete(result, "detailedContent")
	result[pointer.Marker] = replacement
	data[resultField] = encodeJSON(result)
	return true
}

// sourceCopies maps the text of a source that a shell command read to its
// material.
type sourceCopies map[string]string

// maxSourceBytes is the size limit of a file that a shell command read, the
// same limit as for a local spec source.
const maxSourceBytes = 1 << 20

// catCommand finds each cat command in a shell command line.
var catCommand = regexp.MustCompile(`(?:^|[\s;&|()$` + "`" + `])cat(?:\s|$)`)

// catPaths returns the file operands of each cat command in a shell command
// line, in order. Options are skipped. An operand list ends at a shell
// operator or a redirection.
func catPaths(command string) []string {
	var paths []string
	for _, loc := range catCommand.FindAllStringIndex(command, -1) {
		rest := command[loc[1]:]
		for {
			rest = strings.TrimLeft(rest, " \t")
			if rest == "" || strings.ContainsRune(";&|()<>\n`", rune(rest[0])) {
				break
			}
			var operand string
			switch rest[0] {
			case '"', '\'':
				end := strings.IndexByte(rest[1:], rest[0])
				if end < 0 {
					rest = ""
					continue
				}
				operand, rest = rest[1:end+1], rest[end+2:]
			default:
				end := strings.IndexAny(rest, " \t;&|()<>\n`\"'")
				if end < 0 {
					end = len(rest)
				}
				operand, rest = rest[:end], rest[end:]
			}
			if operand != "" && !strings.HasPrefix(operand, "-") {
				paths = append(paths, operand)
			}
		}
	}
	return paths
}

// commandSources reads, for each shell tool call, the files that its command
// read with cat, and keeps those that are stored sources. The file is read at
// the push, so a file that changed after the session matches no source and
// stays inline.
func commandSources(calls streamCalls, sources pointer.Sources) map[string]sourceCopies {
	all := map[string]sourceCopies{}
	for id, tool := range calls.tools {
		if !slices.Contains(commandTools, tool) {
			continue
		}
		copies := sourceCopies{}
		for _, path := range catPaths(jsonString(object(calls.args[id])["command"])) {
			if !filepath.IsAbs(path) {
				if !filepath.IsAbs(calls.cwd) {
					continue
				}
				path = filepath.Join(calls.cwd, path)
			}
			data, ok := readSmallFile(path)
			if !ok {
				continue
			}
			if material, ok := sources[pointer.Digest(data)]; ok {
				copies[string(data)] = material
			}
		}
		if len(copies) > 0 {
			all[id] = copies
		}
	}
	return all
}

// readSmallFile reads a regular, non-empty file of up to maxSourceBytes.
func readSmallFile(path string) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxSourceBytes {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSourceBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxSourceBytes {
		return nil, false
	}
	return data, true
}

// replaceTextCopies splits text into text blocks and pointer blocks, one
// pointer for each copy of a source. A copy must start at the start of a line
// and end at the end of a line.
func replaceTextCopies(text string, copies sourceCopies) ([]json.RawMessage, int) {
	// The longest source first, so that overlapping sources give one result.
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
			if index := sourceOffset(text, source); index >= 0 && (position < 0 || index < position) {
				position, matched = index, source
			}
		}
		if position < 0 {
			blocks = append(blocks, encodeJSON(trace.RawSessionTextBlock{Type: textBlock, Text: text}))
			break
		}
		if position > 0 {
			blocks = append(blocks, encodeJSON(trace.RawSessionTextBlock{Type: textBlock, Text: text[:position]}))
		}
		blocks = append(blocks, pointer.New(copies[matched]))
		text = text[position+len(matched):]
		n++
	}
	if n == 0 {
		return nil, 0
	}
	return blocks, n
}

// sourceOffset returns the offset of the first copy of source in text that
// starts and ends at line boundaries, or -1.
func sourceOffset(text, source string) int {
	if source == "" {
		return -1
	}
	for start := 0; start < len(text); {
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

// replaceWriteArgs returns the arguments of a create call, or of a patch that
// adds one file, with the text that is an exact copy of a source replaced
// with a pointer.
func replaceWriteArgs(tool string, args json.RawMessage, sources pointer.Sources, report pointer.Report) (json.RawMessage, bool) {
	switch tool {
	case createTool:
		fields := object(args)
		if fields == nil || len(fields["file_text"]) == 0 {
			return nil, false
		}
		material, ok := sources[pointer.Digest([]byte(jsonString(fields["file_text"])))]
		if !ok {
			return nil, false
		}
		report.Finder(pointer.FinderFileWrite).BytesRemoved += len(fields["file_text"])
		delete(fields, "file_text")
		fields[pointer.Marker] = pointer.New(material)
		args = encodeJSON(fields)
	case toolApplyPatch:
		// The patch is a JSON string, or an object with an input field, as
		// ReadHookInput reads it.
		path, content, ok := addedFile(patchText(args))
		if !ok {
			return nil, false
		}
		material, ok := sources[pointer.Digest([]byte(content))]
		if !ok {
			return nil, false
		}
		report.Finder(pointer.FinderFileWrite).BytesRemoved += len(args)
		args = encodeJSON(jsonObject{"path": encodeJSON(path), pointer.Marker: pointer.New(material)})
	default:
		return nil, false
	}
	report.Finder(pointer.FinderFileWrite).Replaced++
	return args, true
}

// replaceWrites replaces the text of each successful create call, or patch
// that adds one file, that is an exact copy of a source.
func replaceWrites(data jsonObject, calls streamCalls, sources pointer.Sources, report pointer.Report) bool {
	var requests []jsonObject
	if json.Unmarshal(data["toolRequests"], &requests) != nil {
		return false
	}
	changed := false
	for _, req := range requests {
		if !calls.succeeded[jsonString(req["toolCallId"])] {
			continue
		}
		if args, ok := replaceWriteArgs(jsonString(req["name"]), req["arguments"], sources, report); ok {
			req["arguments"] = args
			changed = true
		}
	}
	if changed {
		data["toolRequests"] = encodeJSON(requests)
	}
	return changed
}

// replaceHookInput replaces the copies in the input of a Trace hook, which
// Copilot logs in a hook.start event: the tool arguments of a preToolUse
// batch, and the arguments and the result text of a postToolUse call. The
// same rules apply as for the tool events.
func replaceHookInput(data jsonObject, calls streamCalls, sources pointer.Sources, report pointer.Report) bool {
	input := object(data["input"])
	if input == nil {
		return false
	}
	changed := false
	switch jsonString(data["hookType"]) {
	case "preToolUse":
		var batch []jsonObject
		if json.Unmarshal(input["toolCalls"], &batch) != nil {
			return false
		}
		for _, call := range batch {
			if !calls.succeeded[jsonString(call["id"])] {
				continue
			}
			if args, ok := replaceWriteArgs(jsonString(call["name"]), call["args"], sources, report); ok {
				call["args"] = args
				changed = true
			}
		}
		if changed {
			input["toolCalls"] = encodeJSON(batch)
		}
	case "postToolUse":
		id := jsonString(input["toolCallId"])
		if !calls.succeeded[id] {
			return false
		}
		tool := jsonString(input["toolName"])
		if args, ok := replaceWriteArgs(tool, input["toolArgs"], sources, report); ok {
			input["toolArgs"] = args
			changed = true
		}
		result := object(input["toolResult"])
		if result != nil && len(result[pointer.Marker]) == 0 {
			if replacement, n := readReplacement(tool, input["toolArgs"], calls.copies[id], jsonString(result["textResultForLlm"]), sources); n > 0 {
				res := report.Finder(pointer.FinderFileRead)
				res.Replaced += n
				res.BytesRemoved += len(result["textResultForLlm"])
				delete(result, "textResultForLlm")
				result[pointer.Marker] = replacement
				input["toolResult"] = encodeJSON(result)
				changed = true
			}
		}
	}
	if changed {
		data["input"] = encodeJSON(input)
	}
	return changed
}

// addedFile returns the path and the content of a patch that adds exactly one
// file. Other patches change content that the patch does not hold, so they
// stay inline.
func addedFile(patch string) (string, string, bool) {
	lines := strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
	if len(lines) < 4 || lines[0] != "*** Begin Patch" || !strings.HasPrefix(lines[1], "*** Add File: ") || lines[len(lines)-1] != "*** End Patch" {
		return "", "", false
	}
	var content strings.Builder
	for _, line := range lines[2 : len(lines)-1] {
		text, ok := strings.CutPrefix(line, "+")
		if !ok {
			return "", "", false
		}
		content.WriteString(text)
		content.WriteByte('\n')
	}
	return strings.TrimPrefix(lines[1], "*** Add File: "), content.String(), true
}
