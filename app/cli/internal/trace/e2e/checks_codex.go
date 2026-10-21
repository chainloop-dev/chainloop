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

package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// codexViewerPointer deliberately ignores source_event and other native
// metadata. A matching digest there alone cannot make a viewer link work.
func codexViewerPointer(lines []json.RawMessage, digest string) bool {
	for _, raw := range lines {
		var entry trace.RawSessionEntry
		if json.Unmarshal(raw, &entry) == nil && jsonPointer(entry.Message.Content, digest) {
			return true
		}
	}
	return false
}

func jsonPointer(raw json.RawMessage, digest string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil && fields != nil {
		var kind, value string
		_ = json.Unmarshal(fields["type"], &kind)
		_ = json.Unmarshal(fields["digest"], &value)
		if kind == pointer.Marker && value == digest {
			return len(fields) == 2
		}
		for _, key := range []string{"content", "source", "input", pointer.Marker} {
			if jsonPointer(fields[key], digest) {
				return true
			}
		}
		return false
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if jsonPointer(block, digest) {
				return true
			}
		}
	}
	return false
}

func checkCodexSpecPointer(s *Session, digest string) error {
	content, err := s.Stored.Material(digest)
	if err != nil {
		return err
	}
	root := s.Checkout
	if root == "" {
		root = s.Workspace.Dir
	}
	original, err := os.ReadFile(filepath.Join(root, ticketPath))
	if err != nil {
		return err
	}
	for _, raw := range evidence(s).RawSession["main"] {
		if inlineSource(raw, content) || inlineSource(raw, original) {
			return errors.New("the Codex transcript retains an inline copy of the stored spec")
		}
	}
	if !codexViewerPointer(evidence(s).RawSession["main"], digest) {
		return errors.New("the Codex viewer message has no spec material pointer")
	}
	return nil
}

func inlineSource(raw json.RawMessage, content []byte) bool {
	if len(content) == 0 {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if bytes.Contains([]byte(text), content) {
			return true
		}
		if json.Valid([]byte(text)) {
			return inlineSource(json.RawMessage(text), content)
		}
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil && fields != nil {
		for _, value := range fields {
			if inlineSource(value, content) {
				return true
			}
		}
		return false
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if inlineSource(block, content) {
				return true
			}
		}
	}
	return false
}

func checkCodexSkillMaterials(s *Session, entry aicodingsession.SpecEntry) error {
	definition, err := s.Stored.Material(entry.Digest)
	if err != nil {
		return fmt.Errorf("read the Codex skill definition: %w", err)
	}
	if !bytes.Contains(definition, []byte(skillName)) || !bytes.Contains(definition, []byte("farewell.txt")) {
		return errors.New("the Codex skill material does not contain its definition")
	}
	if entry.Metadata.PackageDigest == "" {
		return errors.New("the Codex skill has no supporting-file package")
	}
	data, err := s.Stored.Material(entry.Metadata.PackageDigest)
	if err != nil {
		return err
	}
	compressed, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("the Codex skill package has no matching definition")
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) == "SKILL.md" {
			content, err := io.ReadAll(io.LimitReader(archive, 1<<20))
			if err != nil {
				return err
			}
			if !bytes.Equal(content, definition) {
				return errors.New("the Codex skill package differs from its stored definition")
			}
			return nil
		}
	}
}
