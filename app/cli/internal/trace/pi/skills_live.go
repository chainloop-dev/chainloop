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

package pi

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

const piSkillByteOffsetKey = "pi-bytes"

// NewSkillLoads scans the selected live branch for completed loads. The line
// cursor filters uses after a full parse; the byte cursor avoids parsing when
// no complete candidate line was appended.
func (p *Provider) NewSkillLoads(store *state.Store, input *trace.HookInput) []string {
	if store == nil || input == nil || !state.ValidSessionID(input.SessionID) || !filepath.IsAbs(input.TranscriptPath) {
		return nil
	}
	offsets, err := store.SkillScanOffsets(input.SessionID)
	if err != nil {
		offsets = make(map[string]int64)
	}

	byteOffset, initialized := offsets[piSkillByteOffsetKey]
	next, candidate, reset, err := scanSkillTail(input.TranscriptPath, byteOffset)
	if err != nil {
		return nil
	}
	if !initialized {
		candidate = true
	}
	if reset {
		offsets["main"] = 0
	}
	if next == byteOffset && !reset {
		return nil
	}
	if !candidate {
		offsets[piSkillByteOffsetKey] = next
		if err := store.SetSkillScanOffsets(input.SessionID, offsets); err != nil {
			return nil
		}
		return nil
	}

	parsed, err := readSkillSessionPrefix(input.TranscriptPath, input.SessionID, next)
	if err != nil {
		return nil
	}
	var last int64
	for _, entry := range parsed.Selected {
		last = max(last, int64(entry.Line))
	}
	after := offsets["main"]
	if after > last {
		after = 0
	}
	uses := selectedSkills(parsed, after)
	dirs := make([]string, 0, len(uses))
	seen := make(map[string]bool, len(uses))
	for _, use := range uses {
		if seen[use.Dir] {
			continue
		}
		seen[use.Dir] = true
		dirs = append(dirs, use.Dir)
	}
	offsets["main"] = last
	offsets[piSkillByteOffsetKey] = next
	if err := store.SetSkillScanOffsets(input.SessionID, offsets); err != nil {
		return nil
	}
	return dirs
}

func readSkillSessionPrefix(path, sessionID string, size int64) (*parsedSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("read Pi skills: transcript unavailable")
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, size))
	if err != nil {
		return nil, errors.New("read Pi skills: transcript unavailable")
	}

	return parseJSONL(data, sessionID)
}

func scanSkillTail(path string, offset int64) (next int64, candidate, reset bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return offset, false, false, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return offset, false, false, errors.New("read Pi skills: transcript unavailable")
	}
	if info.Size() < offset {
		offset = 0
		reset = true
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, false, reset, err
	}

	next = offset
	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) {
			return next, candidate, reset, nil
		}
		if readErr != nil {
			return offset, false, reset, readErr
		}
		next += int64(len(line))
		if bytes.Contains(line, []byte(`\u`)) ||
			bytes.Contains(line, []byte(`<skill name=`)) ||
			(bytes.Contains(line, []byte(`toolResult`)) && bytes.Contains(line, []byte(`read`))) {
			candidate = true
		}
	}
}
