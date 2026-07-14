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

// Package trufflehog parses TruffleHog's --json output, which is JSONL
// (newline-delimited: one JSON finding object per line), into structured
// findings.
package trufflehog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// CanonicalEmpty is the content crafted for a clean scan (zero findings).
// TruffleHog emits nothing when it finds no secrets, so there is no native
// empty document; storing this keeps a passing scan attestable and round-trips
// through Parse to an empty findings list. The crafter and Parse share this one
// definition so the write and read sides never drift.
var CanonicalEmpty = []byte("[]")

// SourceMetadata mirrors the nested source information TruffleHog attaches to
// each finding. Only the fields needed for policy evaluation are decoded; the
// concrete source shape (Filesystem, Git, ...) varies per source type, so it is
// kept as a raw JSON message to preserve it verbatim without losing type safety
// on the fields we do read.
type SourceMetadata struct {
	Data json.RawMessage `json:"Data,omitempty"`
}

// Finding is a single TruffleHog secret detection result (one JSONL line).
type Finding struct {
	SourceMetadata SourceMetadata `json:"SourceMetadata"`
	SourceID       int            `json:"SourceID"`
	SourceType     int            `json:"SourceType"`
	SourceName     string         `json:"SourceName"`
	DetectorType   int            `json:"DetectorType"`
	DetectorName   string         `json:"DetectorName"`
	DecoderName    string         `json:"DecoderName"`
	Verified       bool           `json:"Verified"`
	Raw            string         `json:"Raw"`
	Redacted       string         `json:"Redacted"`
}

// Parse reads TruffleHog output and returns its findings. It accepts two forms:
//
//   - JSONL: one JSON finding object per line (TruffleHog's native --json
//     output).
//   - JSON array: a single "[...]" document. This is the canonical form we
//     craft for a clean scan (TruffleHog emits nothing when it finds no
//     secrets, so "[]" represents zero findings).
//
// It errors if a non-blank line (JSONL) or the document (array) is not valid
// JSON. An input with no findings (empty, whitespace only, or "[]") yields an
// empty slice and no error; callers decide whether an empty report is
// acceptable.
func Parse(r io.Reader) ([]Finding, error) {
	br := bufio.NewReader(r)

	// Peek at the first non-whitespace byte to distinguish a JSON array
	// document from JSONL without loading the whole (potentially large) stream.
	first, err := firstNonSpaceByte(br)
	if errors.Is(err, io.EOF) {
		return make([]Finding, 0), nil // empty input: zero findings
	}
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(br)

	// A single JSON array document — the canonical form we craft for a clean
	// scan (see CanonicalEmpty), and what jq -s style tooling produces.
	if first == '[' {
		findings := make([]Finding, 0)
		if err := dec.Decode(&findings); err != nil {
			return nil, fmt.Errorf("invalid trufflehog JSON array: %w", err)
		}
		// Reject any non-whitespace content after the array. Without this, a
		// report like "[]" with findings appended afterwards would decode as
		// the empty array and silently ignore the rest, hiding those findings.
		if dec.More() {
			return nil, fmt.Errorf("unexpected content after trufflehog JSON array")
		}
		return findings, nil
	}

	// JSONL: one finding object per line. json.Decoder treats the separating
	// newlines as whitespace, so it streams records natively with no
	// line-length limit and no manual blank-line handling.
	findings := make([]Finding, 0)
	for {
		var f Finding
		err := dec.Decode(&f)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid trufflehog JSONL: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// firstNonSpaceByte consumes leading JSON whitespace and returns the first
// meaningful byte, leaving it unread on the buffered reader so the subsequent
// decoder/scanner sees the full remaining content. Returns io.EOF if the input
// is empty or whitespace only.
func firstNonSpaceByte(br *bufio.Reader) (byte, error) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		if err := br.UnreadByte(); err != nil {
			return 0, err
		}
		return b, nil
	}
}
