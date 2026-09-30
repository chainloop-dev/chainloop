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
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/betterleaks/betterleaks/detect"
	betterleaksregexp "github.com/betterleaks/betterleaks/regexp"
	"github.com/betterleaks/betterleaks/regexp/re2"
	"github.com/betterleaks/betterleaks/sources"
	"golang.org/x/sync/errgroup"
)

const (
	// defaultChunkSize is how much new text each fragment handed to the detector
	// carries. A rule only runs on a fragment that contains one of its keywords,
	// and a whole transcript contains nearly all of them, so scanning it as a
	// single fragment runs nearly every rule over every byte. Small fragments let
	// the keyword prefilter skip most rules. It matches the chunk size betterleaks
	// itself reads files with.
	defaultChunkSize = 100_000

	// chunkOverlapLines is how many trailing lines of a chunk the next chunk
	// scans again. Composite rules only fire when a component is found within a
	// window of lines around the primary match, so the overlap has to cover the
	// widest window in the ruleset or a pair straddling a boundary is missed.
	// TestChunkOverlapCoversComponentWindows keeps it in step with the ruleset.
	chunkOverlapLines = 64

	// attrChunk tags each fragment, and therefore each finding, with the index of
	// its chunk, so that findings can be put back in document order.
	attrChunk = "chainloop.redaction.chunk"
)

// betterleaksScanner detects secrets with the betterleaks default ruleset.
type betterleaksScanner struct {
	// mu serialises Run. The detector keeps per-run state on itself
	// (ValidationCounts is cleared at the start of every Run), so concurrent
	// scans on a shared detector would race. Redaction is not on a hot
	// concurrent path, so serialising is cheaper than owning a detector per
	// caller: constructing one compiles the whole ruleset. A single scan still
	// uses every core, by scanning its chunks concurrently.
	mu       sync.Mutex
	detector *detect.Detector
	// chunkSize is the amount of new text per fragment; see defaultChunkSize.
	chunkSize int
}

var (
	defaultScanner = sync.OnceValues(newBetterleaksScanner)
	// useRE2 selects the regex engine. It is process-wide and only affects
	// patterns compiled afterwards, so it has to run before the ruleset loads.
	useRE2 = sync.OnceFunc(func() { betterleaksregexp.SetEngine(re2.RE2{}) })
)

// DefaultScanner returns the process-wide betterleaks-backed scanner.
// Constructing a detector compiles several hundred regexes and builds a keyword
// trie, so it is built once, lazily, and only for attestations that need it.
func DefaultScanner() (Scanner, error) {
	s, err := defaultScanner()
	if err != nil {
		// Returned bare so that a failure is a nil interface, not a nil pointer
		// wrapped in one.
		return nil, err
	}
	return s, nil
}

func newBetterleaksScanner() (*betterleaksScanner, error) {
	// The library defaults to the standard library engine, which is several
	// times slower on this ruleset; the betterleaks CLI defaults to RE2 for the
	// same reason.
	useRE2()

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

	return &betterleaksScanner{detector: d, chunkSize: defaultChunkSize}, nil
}

func (s *betterleaksScanner) Scan(ctx context.Context, text string) ([]Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := chunkedSource{text: text, chunks: splitLines(text, s.chunkSize, chunkOverlapLines)}

	// Chunks are scanned concurrently, so findings arrive interleaved. They are
	// collected per chunk and joined in chunk order, which keeps the result
	// deterministic: a single goroutine scans each chunk, so a chunk's own
	// findings arrive in the detector's order. That matters because when two
	// rules report the same secret the first finding names its placeholder, and
	// the placeholder is part of the redacted document's digest. Duplicates from
	// the chunk overlap are left for the caller, which deduplicates by secret.
	perChunk := make([][]Finding, len(source.chunks))
	for result := range s.detector.Run(ctx, source) {
		if result.Err != nil {
			return nil, fmt.Errorf("scanning for secrets: %w", result.Err)
		}

		chunk, err := strconv.Atoi(result.Finding.Attr(attrChunk))
		if err != nil || chunk < 0 || chunk >= len(perChunk) {
			return nil, fmt.Errorf("scanning for secrets: finding with an invalid chunk index %q", result.Finding.Attr(attrChunk))
		}

		perChunk[chunk] = appendSecret(perChunk[chunk], result.Finding.RuleID, result.Finding.Secret)

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
				perChunk[chunk] = appendSecret(perChunk[chunk], component.RuleID, component.Secret)
			}
		}
	}

	// Run stops iterating when the context is cancelled, so a partial result
	// here would mean silently under-redacting.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return slices.Concat(perChunk...), nil
}

// appendSecret records a locatable secret. A finding without one cannot be
// searched for in the document, so it is dropped rather than reported.
func appendSecret(dst []Finding, ruleID, secret string) []Finding {
	if secret == "" {
		return dst
	}
	return append(dst, Finding{RuleID: ruleID, Secret: secret})
}

// span is a half-open byte range [start, end) of a text.
type span struct {
	start, end int
}

// splitLines cuts text into chunks that each carry at least size bytes of new
// text, ending at a line end. Each chunk after the first also begins with up to
// overlap trailing lines of its predecessor. Only the last of those may be
// longer than size bytes; the others must fit within size bytes together, so
// that a run of very long lines is not scanned again and again.
//
// A line is never split, even when it is longer than size. That is what keeps a
// secret whole, and it relies on how Redactor renders the document: as indented
// JSON, where a line holds at most one string leaf and newlines inside strings
// are escaped, so no secret it can locate spans a line break.
func splitLines(text string, size, overlap int) []span {
	if len(text) <= size {
		return []span{{0, len(text)}}
	}

	var chunks []span
	for start, pos := 0, 0; pos < len(text); {
		for fresh := pos; pos < len(text) && pos-fresh < size; {
			if nl := strings.IndexByte(text[pos:], '\n'); nl >= 0 {
				pos += nl + 1
			} else {
				pos = len(text)
			}
		}
		chunks = append(chunks, span{start, pos})

		// Walk back over the trailing lines the next chunk re-reads. The last
		// line always is, however long: a composite match on it may have its
		// component at the start of the next chunk.
		start = pos
		for range overlap {
			if start == 0 {
				break
			}
			lineStart := strings.LastIndexByte(text[:start-1], '\n') + 1
			if pos-lineStart > size && start != pos {
				break
			}
			start = lineStart
		}
	}
	return chunks
}

// chunkedSource adapts an in-memory document to the source interface the
// scanner consumes, yielding its chunks from several goroutines at once. Run
// supports that: betterleaks' own file source scans files concurrently the same
// way. Run is the only scanning entry point that is neither deprecated nor
// context-blind, and it takes a source rather than a string.
type chunkedSource struct {
	text   string
	chunks []span
}

func (s chunkedSource) Fragments(ctx context.Context, yield sources.FragmentsFunc) error {
	// The group's context is cancelled once Wait returns, so the caller's is the
	// one checked at the end.
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))

	for i, c := range s.chunks {
		if gctx.Err() != nil {
			break
		}
		g.Go(func() error {
			fragment := sources.Fragment{Raw: s.text[c.start:c.end]}
			fragment.SetAttr(attrChunk, strconv.Itoa(i))
			return yield(fragment, nil)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}
