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
	"crypto/sha256"
	_ "embed"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/betterleaks/betterleaks/config"
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

// rulesConfig is the betterleaks default ruleset, extended with allowlists for
// values the generic rules match but which are not credentials.
//
//go:embed betterleaks.toml
var rulesConfig string

// RulesetVersion identifies what the scanner detects. Bump it whenever
// betterleaks.toml, the betterleaks version, or the handling of findings in
// this file changes what gets redacted, so that policies can tell placeholders
// from different rulesets apart. Version 1 is the plain default ruleset.
const RulesetVersion = 2

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
	return newScannerWithRules(rulesConfig)
}

// newScannerWithRules builds a scanner from a betterleaks TOML configuration.
func newScannerWithRules(rules string) (*betterleaksScanner, error) {
	// The library defaults to the standard library engine, which is several
	// times slower on this ruleset; the betterleaks CLI defaults to RE2 for the
	// same reason.
	useRE2()

	cfg, err := config.ParseTOMLString(rules, "")
	if err != nil {
		return nil, fmt.Errorf("loading the secret scanning rules: %w", err)
	}

	// Validation stays off: validating would reach out to third-party APIs to
	// check whether a candidate credential is live, and crafting a material
	// must not do that.
	d := detect.NewDetectorContext(context.Background(), cfg, detect.ValidationOptions{})

	// The detector checks an allow marker against the whole line of its match.
	// A string leaf of the scanned document is a single line, so one marker
	// would suppress every finding in a whole tool result. Redactor checks the
	// marker against the line of each occurrence instead, when asked to.
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
	perChunk, err := s.scanChunks(ctx, text, splitLines(text, s.chunkSize, chunkOverlapLines))
	if err != nil {
		return nil, err
	}
	return slices.Concat(perChunk...), nil
}

// scanChunks scans the given chunks of text and returns the findings of each
// one, in the order of chunks.
func (s *betterleaksScanner) scanChunks(ctx context.Context, text string, chunks []span) ([][]Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := chunkedSource{text: text, chunks: chunks}

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
		// Both halves have to be redacted. An optional component is context
		// that only raises the confidence of the finding, such as the username
		// next to a generic password, and is not a secret.
		for _, set := range result.Finding.ComponentSets {
			for _, component := range set.Components {
				if component == nil || component.Optional {
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

	return perChunk, nil
}

// forDocument returns a scanner for the passes over a single document.
func (s *betterleaksScanner) forDocument() Scanner {
	return &documentScanner{base: s}
}

// documentScanner scans successive versions of one document, and scans again
// only the chunks whose text changed since the previous version.
//
// Redaction replaces text inside string leaves, which never adds or removes a
// line of the rendered document. So after the first pass the chunks are fixed as
// ranges of lines rather than of bytes, and a chunk far from any replacement has
// exactly the same text as before. The detector is deterministic, so its
// findings for that text are the ones found last time.
type documentScanner struct {
	base *betterleaksScanner
	// chunks are the line ranges the document was cut into on its first scan.
	chunks []lineSpan
	// lines is the number of lines the document had then. A version with a
	// different count is cut afresh.
	lines int
	// findings holds the findings of each chunk of the previous version, by the
	// digest of its text.
	findings map[[sha256.Size]byte][]Finding
}

// lineSpan is a half-open range [start, end) of line indices.
type lineSpan struct {
	start, end int
}

func (d *documentScanner) Scan(ctx context.Context, text string) ([]Finding, error) {
	// Checked here as well as in scanChunks, which a pass whose chunks are all
	// unchanged never reaches.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	lines := strings.Count(text, "\n") + 1
	var chunks []span
	if d.chunks != nil && lines == d.lines {
		chunks = byteSpans(text, d.chunks)
	} else {
		chunks = splitLines(text, d.base.chunkSize, chunkOverlapLines)
		d.chunks, d.lines = lineSpans(text, chunks), lines
	}

	// Hashing the whole document is serial work on every pass otherwise.
	digests := make([][sha256.Size]byte, len(chunks))
	var g errgroup.Group
	g.SetLimit(runtime.GOMAXPROCS(0))
	for i, c := range chunks {
		g.Go(func() error {
			digests[i] = sha256.Sum256([]byte(text[c.start:c.end]))
			return nil
		})
	}
	_ = g.Wait()

	perChunk := make([][]Finding, len(chunks))
	var stale []int
	for i := range chunks {
		if found, ok := d.findings[digests[i]]; ok {
			perChunk[i] = found
			continue
		}
		stale = append(stale, i)
	}

	if len(stale) > 0 {
		toScan := make([]span, len(stale))
		for j, i := range stale {
			toScan[j] = chunks[i]
		}
		scanned, err := d.base.scanChunks(ctx, text, toScan)
		if err != nil {
			return nil, err
		}
		for j, i := range stale {
			perChunk[i] = scanned[j]
		}
	}

	// Only the previous version is ever compared against, so older findings
	// are dropped rather than kept for the life of the document.
	d.findings = make(map[[sha256.Size]byte][]Finding, len(chunks))
	for i := range chunks {
		d.findings[digests[i]] = perChunk[i]
	}

	return slices.Concat(perChunk...), nil
}

// lineSpans converts byte spans of text, each starting at a line start, into
// line ranges.
func lineSpans(text string, spans []span) []lineSpan {
	out := make([]lineSpan, len(spans))
	// Spans are ordered and may overlap, so the line count is carried forward
	// from the previous span's start rather than recounted from the beginning.
	offset, line := 0, 0
	for i, s := range spans {
		line += strings.Count(text[offset:s.start], "\n")
		offset = s.start
		end := line + strings.Count(text[s.start:s.end], "\n")
		if s.end == len(text) && !strings.HasSuffix(text, "\n") {
			// The final line has no newline to count.
			end++
		}
		out[i] = lineSpan{start: line, end: end}
	}
	return out
}

// byteSpans converts line ranges back into byte spans of text. A range whose
// last line has no newline runs to the end of the text.
func byteSpans(text string, ranges []lineSpan) []span {
	out := make([]span, len(ranges))
	offset, line := 0, 0
	// advance moves offset to the start of the given line.
	advance := func(to int) {
		for ; line < to; line++ {
			nl := strings.IndexByte(text[offset:], '\n')
			if nl < 0 {
				offset = len(text)
				line = to
				return
			}
			offset += nl + 1
		}
	}
	for i, r := range ranges {
		advance(r.start)
		start := offset
		advance(r.end)
		out[i] = span{start: start, end: offset}
		// The next range starts at or before this end, so rewind.
		line, offset = r.start, start
	}
	return out
}

// appendSecret records a locatable secret. A finding without one cannot be
// searched for in the document, so it is dropped rather than reported.
func appendSecret(dst []Finding, ruleID, secret string) []Finding {
	switch ruleID {
	case jwtRuleID:
		secret = cutAtTerminatorEscape(secret)
	case genericPasswordRuleID:
		secret = cutAtLineEscape(secret)
	}
	if secret == "" {
		return dst
	}
	return append(dst, Finding{RuleID: ruleID, Secret: secret})
}

const (
	// jwtRuleID is the default ruleset's rule for JSON Web Tokens.
	jwtRuleID = "jwt"
	// genericPasswordRuleID is the default ruleset's rule for a password
	// assigned to a password key. Its unquoted value stops at whitespace in
	// source text, but in JSON-encoded text a line break is the escape `\n`, so
	// the match runs into the next line. That swallowed, for instance, the line
	// break after a placeholder the transcript already showed, and reported the
	// placeholder as a new secret.
	genericPasswordRuleID = "generic-password"
)

// cutAtTerminatorEscape removes the text from the first whitespace or quote
// escape onwards.
//
// The jwt rule allows a backslash in its last two segments, so in JSON-encoded
// text its match runs through an escape such as `\n` and into the word after
// it. Replacing that match would remove text that is not part of the token. A
// JWT is base64url, so the part before such an escape is the whole token. Other
// escapes, such as `\/` or `\\`, are kept: they are not proof that the token has
// ended, and keeping them can only over-redact.
//
// Escapes are read as units from the left, so the second backslash of a `\\`
// escape is never taken as the start of another escape.
func cutAtTerminatorEscape(secret string) string {
	return cutAtEscape(secret, `"nrt`)
}

// cutAtLineEscape removes the text from the first whitespace escape onwards. A
// password is not cut at a quote escape: its value often starts with one,
// `password: \"...\"`, which is part of it.
func cutAtLineEscape(secret string) string {
	return cutAtEscape(secret, "nrt")
}

// cutAtEscape removes the text from the first escape of one of the given
// characters onwards.
func cutAtEscape(secret, terminators string) string {
	for i := 0; i+1 < len(secret); i++ {
		if secret[i] != '\\' {
			continue
		}
		if strings.IndexByte(terminators, secret[i+1]) >= 0 {
			return secret[:i]
		}
		// Skip the escaped character, so that it is not read as the start of
		// an escape.
		i++
	}
	return secret
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
