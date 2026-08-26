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

package redaction

import (
	"context"
	"fmt"
	"sync"

	"github.com/betterleaks/betterleaks/detect"
	"github.com/betterleaks/betterleaks/sources"
)

// betterleaksScanner detects secrets with the betterleaks default ruleset.
type betterleaksScanner struct {
	// mu serialises Run. The detector keeps per-run state on itself
	// (ValidationCounts is cleared at the start of every Run), so concurrent
	// scans on a shared detector would race. Redaction is not on a hot
	// concurrent path, so serialising is cheaper than owning a detector per
	// caller: constructing one compiles the whole ruleset.
	mu       sync.Mutex
	detector *detect.Detector
}

var defaultScanner = sync.OnceValues(newBetterleaksScanner)

// DefaultScanner returns the process-wide betterleaks-backed scanner.
// Constructing a detector compiles several hundred regexes and builds a keyword
// trie, so it is built once, lazily, and only for attestations that need it.
func DefaultScanner() (Scanner, error) {
	return defaultScanner()
}

func newBetterleaksScanner() (Scanner, error) {
	// Validation stays off, which is what the default constructor gives us:
	// validating would reach out to third-party APIs to check whether a candidate
	// credential is live, and crafting a material must not do that.
	d, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("loading the default secret scanning rules: %w", err)
	}

	// A session transcript is text the model was free to write, so an in-band
	// "betterleaks:allow" or "gitleaks:allow" must not switch redaction off.
	d.IgnoreGitleaksAllow = true
	// Findings must carry the verbatim secret: it is what we search for in the
	// document in order to replace it.
	d.Redact = 0
	// Recursive decoding would report the *decoded* secret, which is not a
	// substring of the document and therefore cannot be located and replaced.
	// Encoded secrets are a known gap.
	d.MaxDecodeDepth = 0
	// Not a size guard: an oversized fragment is silently *skipped*, which would
	// mean silent under-redaction. The size cap lives in Redactor and fails
	// closed instead.
	d.MaxTargetMegaBytes = 0
	// Run otherwise accumulates every finding onto the detector for the benefit
	// of a deprecated accessor we do not use. Since this detector is a
	// process-wide singleton scanned against repeatedly, that would grow without
	// bound. Results are read from the Run iterator instead.
	d.SkipFindingAppend = true

	return &betterleaksScanner{detector: d}, nil
}

func (s *betterleaksScanner) Scan(ctx context.Context, text string) ([]Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var findings []Finding
	for result := range s.detector.Run(ctx, stringSource{text: text}) {
		if result.Err != nil {
			return nil, fmt.Errorf("scanning for secrets: %w", result.Err)
		}

		findings = appendSecret(findings, result.Finding.RuleID, result.Finding.Secret)

		// Composite rules report only their primary match. An AWS access key id,
		// for instance, only matches when a secret access key is found near it,
		// and the rule for that component is marked as not independently
		// reportable — so the primary finding names the harmless public
		// identifier while the actual credential arrives only as a component.
		// Both halves have to be redacted.
		for _, set := range result.Finding.ComponentSets {
			for _, component := range set.Components {
				if component == nil {
					continue
				}
				findings = appendSecret(findings, component.RuleID, component.Secret)
			}
		}
	}

	// Run stops iterating when the context is cancelled, so a partial result
	// here would mean silently under-redacting.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return findings, nil
}

// appendSecret records a locatable secret. A finding without one cannot be
// searched for in the document, so it is dropped rather than reported.
func appendSecret(dst []Finding, ruleID, secret string) []Finding {
	if secret == "" {
		return dst
	}
	return append(dst, Finding{RuleID: ruleID, Secret: secret})
}

// stringSource adapts an in-memory document to the source interface the scanner
// consumes. Run is the only scanning entry point that is neither deprecated nor
// context-blind, and it takes a source rather than a string.
type stringSource struct {
	text string
}

func (s stringSource) Fragments(ctx context.Context, yield sources.FragmentsFunc) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return yield(sources.Fragment{Raw: s.text}, nil)
}
