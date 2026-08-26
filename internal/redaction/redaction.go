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

// Package redaction removes detected secrets from JSON documents before they
// leave the machine.
//
// Detection and rewriting are deliberately split. Secrets are detected against
// the document rendered as indented JSON, because the patterns that catch
// generic credentials rely on seeing a key next to its value. The rewrite then
// happens structurally, on the decoded value tree, so the output cannot be
// malformed JSON. A convergence loop re-scans after every rewrite, which makes
// idempotency a property of the algorithm rather than of the placeholder format.
package redaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	// ErrNotConverged is returned when repeated passes keep detecting secrets,
	// which in practice means a placeholder is itself matching a rule.
	ErrNotConverged = errors.New("secret redaction did not converge")
	// ErrInvalidJSON is returned when the document is not a single JSON object.
	ErrInvalidJSON = errors.New("document is not a JSON object")
	// ErrDuplicateKey is returned when an object repeats a key. Decoding keeps
	// only the last value for a repeated key, so a secret in an earlier one would
	// never be scanned; redaction refuses the document rather than pass it
	// through unexamined.
	ErrDuplicateKey = errors.New("document contains a duplicate object key")
)

// DefaultMaxPasses bounds the convergence loop. A successful redaction costs two
// passes: one that finds the secrets and one that confirms none are left.
//
// Redaction is deliberately unbounded in size and time. A session that cannot be
// scanned cannot be stored, and failing an attestation over a large-but-honest
// transcript is worse than taking a while over it. Callers that need a bound can
// impose one through the context they pass to Redact.
const DefaultMaxPasses = 4

// Finding is a located secret. It is deliberately decoupled from any particular
// scanning engine's types.
type Finding struct {
	// RuleID names the rule that matched. It ends up in the placeholder, so it
	// must never contain secret material.
	RuleID string
	// Secret is the matched credential, verbatim as it appeared in the scanned
	// text.
	Secret string
}

// Scanner detects secrets in a text fragment.
type Scanner interface {
	Scan(ctx context.Context, text string) ([]Finding, error)
}

// PathFilter reports whether the string leaf at the given path may be rewritten.
// Paths look like "/data/raw_session/main/0/content": a leading slash, object
// keys and array indices separated by slashes.
type PathFilter func(path string) bool

// Report summarises a Redact call. It never contains secret material, so it is
// safe to log and to surface as material annotations.
type Report struct {
	// Replacements is the total number of substitutions performed.
	Replacements int
	// ByRule counts substitutions per rule id.
	ByRule map[string]int
	// Unlocated counts, per rule id, secrets the scanner reported but which
	// could not be attributed to any eligible leaf. That happens when the match
	// lies in a path the filter protects, or when it spans the JSON punctuation
	// between two adjacent leaves.
	Unlocated map[string]int
	// Passes is the number of detection passes performed.
	Passes int
}

// Changed reports whether anything was redacted.
func (r *Report) Changed() bool { return r != nil && r.Replacements > 0 }

// RuleIDs returns the sorted, deduplicated ids of the rules that actually
// redacted something.
func (r *Report) RuleIDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.ByRule))
	for id := range r.ByRule {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Redactor rewrites JSON documents, replacing detected secrets with
// placeholders. It is safe for concurrent use as long as the Scanner is.
type Redactor struct {
	scanner       Scanner
	pathFilter    PathFilter
	maxPasses     int
	placeholder   func(ruleID string) string
	isPlaceholder func(string) bool
}

// Option customises a Redactor.
type Option func(*Redactor)

// WithPathFilter restricts which string leaves may be rewritten. The default
// allows every leaf.
func WithPathFilter(f PathFilter) Option {
	return func(r *Redactor) {
		if f != nil {
			r.pathFilter = f
		}
	}
}

// WithMaxPasses bounds the convergence loop.
func WithMaxPasses(n int) Option {
	return func(r *Redactor) {
		if n > 0 {
			r.maxPasses = n
		}
	}
}

// WithPlaceholder overrides the replacement text derived from a rule id.
//
// The two functions are supplied together on purpose. Some rules match a
// position rather than a value, so they report a placeholder as though it were a
// secret; recognising our own output is what stops redaction from rewriting it
// and keeps redacting an already-redacted document a no-op. A format without a
// matching recogniser would silently lose that.
func WithPlaceholder(format func(ruleID string) string, matches func(string) bool) Option {
	return func(r *Redactor) {
		if format != nil {
			r.placeholder = format
		}
		if matches != nil {
			r.isPlaceholder = matches
		}
	}
}

// New builds a Redactor around the given Scanner.
func New(s Scanner, opts ...Option) *Redactor {
	r := &Redactor{
		scanner:       s,
		pathFilter:    func(string) bool { return true },
		maxPasses:     DefaultMaxPasses,
		placeholder:   DefaultPlaceholder,
		isPlaceholder: IsDefaultPlaceholder,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// DefaultPlaceholder is the replacement text for a redacted secret. It is
// deterministic, so redacting the same document twice yields the same digest,
// and it names the rule so a reviewer can tell what kind of credential was
// present without being able to recover it.
func DefaultPlaceholder(ruleID string) string {
	if ruleID == "" {
		return "[REDACTED]"
	}
	return "[REDACTED:" + ruleID + "]"
}

// defaultPlaceholderPattern recognises the output of DefaultPlaceholder for any
// rule id, including ids this build has never seen.
var defaultPlaceholderPattern = regexp.MustCompile(`^\[REDACTED(?::[^\]\s]*)?\]$`)

// IsDefaultPlaceholder reports whether s is a placeholder DefaultPlaceholder
// could have produced.
func IsDefaultPlaceholder(s string) bool {
	return defaultPlaceholderPattern.MatchString(s)
}

// Redact returns a copy of doc with every detected secret replaced. When
// nothing is detected the input is returned verbatim, so that a document
// without secrets keeps its original digest.
func (r *Redactor) Redact(ctx context.Context, doc []byte) ([]byte, *Report, error) {
	if r.scanner == nil {
		return nil, nil, errors.New("no scanner configured")
	}
	root, err := decodeObject(doc)
	if err != nil {
		return nil, nil, err
	}

	report := &Report{ByRule: map[string]int{}, Unlocated: map[string]int{}}
	// Secrets no eligible leaf contains, so that the loop stops chasing them.
	skip := make(map[string]struct{})
	converged := false

	for pass := 1; pass <= r.maxPasses; pass++ {
		report.Passes = pass

		// Indented rendering is what makes generic key/value rules work: it puts
		// each pair on its own line, which is also the shape the scanner's own
		// per-line deduplication assumes.
		text, err := encode(root, true)
		if err != nil {
			return nil, nil, fmt.Errorf("rendering document: %w", err)
		}

		findings, err := r.scanner.Scan(ctx, text)
		if err != nil {
			return nil, nil, fmt.Errorf("scanning for secrets: %w", err)
		}

		pending := pendingSecrets(findings, skip, r.isPlaceholder)
		if len(pending) == 0 {
			converged = true
			break
		}

		w := &rewriter{
			pathFilter:  r.pathFilter,
			placeholder: r.placeholder,
			secrets:     pending,
			byRule:      map[string]int{},
			located:     map[string]struct{}{},
		}
		w.rewriteMap(root, "")

		for _, s := range pending {
			if _, ok := w.located[s.secret]; !ok {
				skip[s.secret] = struct{}{}
				report.Unlocated[s.ruleID]++
			}
		}
		for rule, n := range w.byRule {
			report.ByRule[rule] += n
		}
		report.Replacements += w.count

		if w.count == 0 {
			// Nothing could be rewritten, so a further pass would see the same
			// document and report the same findings.
			converged = true
			break
		}
	}

	if !converged {
		return nil, nil, fmt.Errorf("%w after %d passes: a placeholder is likely being detected as a secret", ErrNotConverged, r.maxPasses)
	}

	if report.Replacements == 0 {
		// Hand back the original bytes: re-encoding would reorder object keys and
		// change the artifact digest for no benefit.
		return doc, report, nil
	}

	out, err := encode(root, false)
	if err != nil {
		return nil, nil, fmt.Errorf("re-encoding redacted document: %w", err)
	}
	return []byte(out), report, nil
}

// secretRule pairs a secret with the rule that matched it.
type secretRule struct {
	secret string
	ruleID string
}

// pendingSecrets deduplicates findings and drops the ones the loop has decided
// not to chase. The result is ordered longest-secret-first so that a secret
// contained within a longer one cannot partially clobber it.
func pendingSecrets(findings []Finding, skip map[string]struct{}, isPlaceholder func(string) bool) []secretRule {
	seen := make(map[string]struct{}, len(findings))
	out := make([]secretRule, 0, len(findings))

	for _, f := range findings {
		if f.Secret == "" {
			continue
		}
		if _, skipped := skip[f.Secret]; skipped {
			continue
		}
		// A finding whose whole secret is a placeholder is a rule matching the
		// position it sits in, not a credential. Rewriting it would churn the
		// document on every run and never terminate.
		if isPlaceholder != nil && isPlaceholder(f.Secret) {
			continue
		}
		if _, dup := seen[f.Secret]; dup {
			continue
		}
		seen[f.Secret] = struct{}{}
		out = append(out, secretRule{secret: f.Secret, ruleID: f.RuleID})
	}

	sort.Slice(out, func(i, j int) bool {
		if len(out[i].secret) != len(out[j].secret) {
			return len(out[i].secret) > len(out[j].secret)
		}
		return out[i].secret < out[j].secret
	})
	return out
}

// rewriter walks a decoded JSON value tree replacing secrets in eligible string
// leaves.
type rewriter struct {
	pathFilter  PathFilter
	placeholder func(string) string
	secrets     []secretRule
	byRule      map[string]int
	located     map[string]struct{}
	count       int
}

func (w *rewriter) rewriteMap(m map[string]any, path string) {
	for k, child := range m {
		m[k] = w.rewrite(child, path+"/"+k)
	}
}

func (w *rewriter) rewrite(node any, path string) any {
	switch v := node.(type) {
	case map[string]any:
		w.rewriteMap(v, path)
		return v
	case []any:
		for i, child := range v {
			v[i] = w.rewrite(child, path+"/"+strconv.Itoa(i))
		}
		return v
	case string:
		if !w.pathFilter(path) {
			return v
		}
		return w.redactLeaf(v)
	default:
		// Numbers, booleans and null cannot carry a secret the scanner reported
		// as a string.
		return node
	}
}

// redactLeaf substitutes secrets inside a single string leaf. Matching happens
// against the leaf's JSON-encoded form, because that is the text the scanner
// saw: a secret containing a newline, for instance, reaches us as the two
// characters `\n`.
func (w *rewriter) redactLeaf(s string) string {
	body, err := encodeStringBody(s)
	if err != nil {
		return s
	}

	var (
		n        int
		lastRule string
	)
	for _, sr := range w.secrets {
		c := strings.Count(body, sr.secret)
		if c == 0 {
			continue
		}
		body = strings.ReplaceAll(body, sr.secret, w.placeholder(sr.ruleID))
		n += c
		w.byRule[sr.ruleID] += c
		w.located[sr.secret] = struct{}{}
		lastRule = sr.ruleID
	}
	if n == 0 {
		return s
	}
	w.count += n

	var out string
	if err := json.Unmarshal([]byte(`"`+body+`"`), &out); err != nil {
		// A replacement cut through a JSON escape sequence. Drop the whole leaf
		// rather than risk leaving part of the secret behind.
		return w.placeholder(lastRule)
	}
	return out
}

// decodeObject parses doc into a value tree, keeping numbers in their original
// textual form so re-encoding does not reformat them.
func decodeObject(doc []byte) (map[string]any, error) {
	// Checked before decoding, because decoding is what loses the information.
	if err := rejectDuplicateKeys(doc); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()

	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	// Decoder.More is not a top-level end-of-input check: it reports false for a
	// trailing "]" or "}", which would let a malformed document through. Require
	// the input to be exhausted instead.
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: unexpected trailing content", ErrInvalidJSON)
	}

	obj, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: expected an object at the document root", ErrInvalidJSON)
	}
	return obj, nil
}

// jsonFrame tracks one open object or array while walking a token stream.
type jsonFrame struct {
	isObject bool
	keys     map[string]struct{}
	// expectKey is true when the next token in an object is a key rather than a
	// value.
	expectKey bool
}

// rejectDuplicateKeys reports an error if any object in doc repeats a key.
//
// Decoding into a map keeps only the last value for a repeated key. A secret in
// an earlier one would therefore never reach the scanner, and because nothing
// was replaced the original bytes — secret included — would be handed back as
// "clean". Duplicate keys have no legitimate meaning in evidence, so the
// document is refused.
func rejectDuplicateKeys(doc []byte) error {
	dec := json.NewDecoder(bytes.NewReader(doc))
	var stack []*jsonFrame

	top := func() *jsonFrame {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}

	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
		}

		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, &jsonFrame{isObject: true, keys: map[string]struct{}{}, expectKey: true})
			case '[':
				stack = append(stack, &jsonFrame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("%w: unbalanced %q", ErrInvalidJSON, delim)
				}
				stack = stack[:len(stack)-1]
				// The nested value is complete, so the parent object expects the
				// next key.
				if parent := top(); parent != nil && parent.isObject {
					parent.expectKey = true
				}
			}
			continue
		}

		frame := top()
		if frame == nil || !frame.isObject {
			// A top-level scalar or an array element: no keys involved.
			continue
		}

		if !frame.expectKey {
			frame.expectKey = true
			continue
		}

		key, _ := token.(string)
		if _, duplicate := frame.keys[key]; duplicate {
			return fmt.Errorf("%w: %q", ErrDuplicateKey, key)
		}
		frame.keys[key] = struct{}{}
		frame.expectKey = false
	}
}

// encode serialises v. HTML escaping is disabled so transcript text keeps its
// angle brackets and ampersands instead of being mangled into \u sequences.
func encode(v any, indent bool) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// encodeStringBody returns the JSON encoding of s without the surrounding
// quotes, using the same escaping rules as encode.
func encodeStringBody(s string) (string, error) {
	encoded, err := encode(s, false)
	if err != nil {
		return "", err
	}
	if len(encoded) < 2 {
		return "", fmt.Errorf("unexpected encoding %q for a string leaf", encoded)
	}
	return encoded[1 : len(encoded)-1], nil
}
