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

package aicodingsession

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chainloop-dev/chainloop/internal/redaction"
	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
)

// protectedPaths are the string leaves redaction must never rewrite: the
// identifiers, enumerations, timestamps and file paths that the platform joins
// records on and that policies read. "*" matches a single array index or object
// key.
//
// Everything not listed is eligible, which notably includes the whole
// raw_session transcript, warnings, the repository URL (a common place for
// embedded credentials), and any field a future schema version introduces.
//
// A deny-list rather than an allow-list is deliberate. An allow-list would
// silently stop scanning every newly added free-form field, and a silent gap in
// a redaction control is far worse than an over-eager match, which is at least
// counted in the report and published as an annotation.
var protectedPaths = []string{
	"/chainloop.material.evidence.id",
	"/schema",
	"/data/schema_version",
	"/data/agent/name",
	"/data/agent/version",
	"/data/session/id",
	"/data/session/slug",
	"/data/session/started_at",
	"/data/session/ended_at",
	"/data/git_context/branch",
	"/data/git_context/commit_start",
	"/data/git_context/commit_end",
	"/data/git_context/commits/*",
	"/data/code_changes/files/*/path",
	"/data/code_changes/files/*/status",
	"/data/code_changes/files/*/attribution",
	"/data/code_changes/files/*/session_ids/*",
	"/data/model/primary",
	"/data/model/provider",
	"/data/model/models_used/*",
	"/data/tools_used/summary/*/tool_name",
	"/data/subagents/*/id",
	"/data/subagents/*/type",
}

// Redact removes detected secrets from an AI coding session evidence document.
//
// It returns the sanitised bytes and a summary of what was replaced. A document
// with no detected secrets is returned verbatim, so that the common case keeps
// its original digest. The result is guaranteed to still validate against the AI
// coding session schema; if it does not, redaction fails rather than uploading
// either an invalid document or an unredacted one.
func Redact(ctx context.Context, evidence []byte) ([]byte, *redaction.Report, error) {
	scanner, err := redaction.DefaultScanner()
	if err != nil {
		return nil, nil, fmt.Errorf("initialising the secret scanner: %w", err)
	}

	redacted, report, err := redaction.New(scanner, redaction.WithPathFilter(eligible)).Redact(ctx, evidence)
	if err != nil {
		return nil, nil, err
	}

	if report.Changed() {
		if err := validate(redacted); err != nil {
			return nil, nil, fmt.Errorf("redacting secrets produced an invalid AI coding session: %w", err)
		}
	}

	return redacted, report, nil
}

// eligible reports whether the string leaf at path may be rewritten.
func eligible(path string) bool {
	for _, pattern := range protectedPaths {
		if matchPath(pattern, path) {
			return false
		}
	}
	return true
}

// matchPath compares a slash-separated path against a pattern in which "*"
// stands for exactly one segment.
func matchPath(pattern, path string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")
	if len(patternSegments) != len(pathSegments) {
		return false
	}
	for i, want := range patternSegments {
		if want != "*" && want != pathSegments[i] {
			return false
		}
	}
	return true
}

// validate re-checks the redacted document against the AI coding session schema,
// mirroring what the crafter validates on the way in.
func validate(evidence []byte) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(evidence, &envelope); err != nil {
		return fmt.Errorf("decoding the evidence envelope: %w", err)
	}

	// Decoded generically because that is what the JSON schema validator
	// consumes; the crafter keeps using the typed Data struct for everything else.
	var data any
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return fmt.Errorf("decoding the data field: %w", err)
	}

	return schemavalidators.ValidateAICodingSession(data, schemavalidators.AICodingSessionVersion0_1)
}
