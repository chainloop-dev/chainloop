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
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeScanner reports a fixed set of findings, so the engine's walk, rewrite and
// convergence behaviour can be tested without depending on the real ruleset.
type fakeScanner struct {
	findings []Finding
	// requirePresent only reports findings whose secret actually appears in the
	// scanned text, which is how a real detector behaves.
	requirePresent bool
	calls          int
}

func (f *fakeScanner) Scan(ctx context.Context, text string) ([]Finding, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !f.requirePresent {
		return f.findings, nil
	}
	var out []Finding
	for _, fi := range f.findings {
		if strings.Contains(text, fi.Secret) {
			out = append(out, fi)
		}
	}
	return out, nil
}

// fakeDocumentScanner binds a new fakeScanner to every document, to check how
// Redact uses a scanner that can rescan incrementally. Its own Scan must never
// be called once Redact has bound a document scanner.
type fakeDocumentScanner struct {
	fakeScanner
	bound []*fakeScanner
}

func (f *fakeDocumentScanner) forDocument() Scanner {
	s := &fakeScanner{findings: f.findings, requirePresent: f.requirePresent}
	f.bound = append(f.bound, s)
	return s
}

// TestRedactBindsOneScannerPerDocument checks that every pass over a document
// goes through the scanner bound to it, and that each Redact binds a new one:
// what that scanner remembers belongs to a single document.
func TestRedactBindsOneScannerPerDocument(t *testing.T) {
	scanner := &fakeDocumentScanner{fakeScanner: fakeScanner{
		findings:       []Finding{{RuleID: "r1", Secret: "SEC"}},
		requirePresent: true,
	}}
	r := New(scanner)

	for i := 1; i <= 2; i++ {
		_, report, err := r.Redact(context.Background(), []byte(`{"a":"x SEC x"}`))
		require.NoError(t, err)
		assert.Equal(t, 2, report.Passes)
		require.Len(t, scanner.bound, i, "each document gets its own scanner")
		assert.Equal(t, 2, scanner.bound[i-1].calls, "both passes use the scanner bound to the document")
	}
	assert.Zero(t, scanner.calls, "the unbound scanner is never used")
}

func TestRedact(t *testing.T) {
	testCases := []struct {
		name     string
		doc      string
		findings []Finding
		opts     []Option
		// reportAlways makes the fake scanner report its findings even when the
		// secret does not appear in the scanned text, to exercise the engine's
		// handling of findings it cannot attribute to any leaf.
		reportAlways     bool
		wantErr          error
		wantUnchanged    bool // output must be byte-identical to the input
		wantReplacements int
		wantByRule       map[string]int
		wantUnlocated    map[string]int
		mustNotContain   []string
		mustContain      []string
	}{
		{
			name:          "no findings returns the input verbatim",
			doc:           `{"data":{"a":"hello world","n":1}}`,
			wantUnchanged: true,
		},
		{
			name:             "secret in a nested leaf is replaced",
			doc:              `{"data":{"raw_session":{"main":[{"content":"run FAKE-AWS-KEY-NOT-A-REAL-PATTERN now"}]},"keep":"untouched"}}`,
			findings:         []Finding{{RuleID: "aws-access-token", Secret: "FAKE-AWS-KEY-NOT-A-REAL-PATTERN"}},
			wantReplacements: 1,
			wantByRule:       map[string]int{"aws-access-token": 1},
			mustNotContain:   []string{"FAKE-AWS-KEY-NOT-A-REAL-PATTERN"},
			mustContain:      []string{"[REDACTED:aws-access-token]", "untouched", "run ", " now"},
		},
		{
			name:             "same secret across three leaves",
			doc:              `{"a":"x SEC x","b":{"c":"SEC"},"d":["SEC"]}`,
			findings:         []Finding{{RuleID: "r1", Secret: "SEC"}},
			wantReplacements: 3,
			wantByRule:       map[string]int{"r1": 3},
			mustNotContain:   []string{`"SEC"`},
		},
		{
			name:             "secret twice in one leaf",
			doc:              `{"a":"SEC and SEC again"}`,
			findings:         []Finding{{RuleID: "r1", Secret: "SEC"}},
			wantReplacements: 2,
			wantByRule:       map[string]int{"r1": 2},
			mustNotContain:   []string{"SEC and"},
		},
		{
			name:             "escapes and non-ascii survive",
			doc:              `{"a":"line1\nSEC\ttab \"quoted\" <b> café 🚀"}`,
			findings:         []Finding{{RuleID: "r1", Secret: "SEC"}},
			wantReplacements: 1,
			mustNotContain:   []string{`\u003c`, `"SEC`},
			mustContain:      []string{`line1\n`, `\ttab`, `\"quoted\"`, "<b>", "café", "🚀"},
		},
		{
			name:             "severed escape sequence drops the whole leaf",
			doc:              `{"a":"before\nSEC after"}`,
			findings:         []Finding{{RuleID: "r1", Secret: "nSEC"}},
			wantReplacements: 1,
			mustNotContain:   []string{"before", "after"},
			mustContain:      []string{"[REDACTED:r1]"},
		},
		{
			name:             "secret ending on the backslash of an escape keeps the leaf",
			doc:              `{"a":"{\"url\":\"https://h/x?sig=SEC\"}"}`,
			findings:         []Finding{{RuleID: "r1", Secret: `SEC\`}},
			wantReplacements: 1,
			wantByRule:       map[string]int{"r1": 1},
			mustContain:      []string{`{"a":"{\"url\":\"https://h/x?sig=[REDACTED:r1]\"}"}`},
		},
		{
			name:             "secret ending on a complete escaped backslash keeps it",
			doc:              `{"a":"SEC\\ after"}`,
			findings:         []Finding{{RuleID: "r1", Secret: `SEC\\`}},
			wantReplacements: 1,
			wantByRule:       map[string]int{"r1": 1},
			mustContain:      []string{`{"a":"[REDACTED:r1] after"}`},
		},
		{
			name:     "protected path is left alone and recorded",
			doc:      `{"keepme":"SEC","other":"plain"}`,
			findings: []Finding{{RuleID: "r1", Secret: "SEC"}},
			opts: []Option{WithPathFilter(func(p string) bool {
				return p != "/keepme"
			})},
			wantUnchanged: true,
			wantUnlocated: map[string]int{"r1": 1},
		},
		{
			name:     "a secret in a protected and an eligible leaf is replaced only where eligible",
			doc:      `{"keepme":"SEC","other":"x SEC"}`,
			findings: []Finding{{RuleID: "r1", Secret: "SEC"}},
			opts: []Option{WithPathFilter(func(p string) bool {
				return p != "/keepme"
			})},
			wantReplacements: 1,
			wantByRule:       map[string]int{"r1": 1},
			// The copy left in the protected leaf is still found on the next
			// pass, where no eligible leaf holds it any more.
			wantUnlocated: map[string]int{"r1": 1},
			mustContain:   []string{`"keepme":"SEC"`, `"other":"x [REDACTED:r1]"`},
		},
		{
			name:          "finding present nowhere is classified as an artifact",
			doc:           `{"a":"plain"}`,
			findings:      []Finding{{RuleID: "r1", Secret: "NOT-IN-DOC"}},
			reportAlways:  true,
			wantUnchanged: true,
			wantUnlocated: map[string]int{"r1": 1},
		},
		{
			name:     "placeholder that keeps matching does not converge",
			doc:      `{"a":"SEC"}`,
			findings: []Finding{{RuleID: "r1", Secret: "SEC"}},
			// A custom placeholder with no recogniser: the engine cannot tell its
			// own output apart from a secret, so it rewrites it forever. This is
			// the backstop that stops that being an infinite loop.
			opts: []Option{WithPlaceholder(func(string) string {
				return "SEC"
			}, nil)},
			wantErr: ErrNotConverged,
		},
		{
			// 1e400 is valid JSON but out of float64 range: every decoder on the
			// path has to keep numbers in their textual form, or an honest document
			// gets refused over a number nobody was going to scan anyway.
			name:             "numbers keep their exact representation",
			doc:              `{"a":"SEC","big":12345678901234567890,"exp":1e10,"f":0.30000000000000004,"huge":1e400}`,
			findings:         []Finding{{RuleID: "r1", Secret: "SEC"}},
			wantReplacements: 1,
			mustContain:      []string{"12345678901234567890", "1e10", "0.30000000000000004", "1e400"},
		},
		{
			name:    "not json",
			doc:     `not json at all`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "json array root is rejected",
			doc:     `["a"]`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "json null root is rejected",
			doc:     `null`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "trailing content is rejected",
			doc:     `{"a":"b"} trailing`,
			wantErr: ErrInvalidJSON,
		},
		{
			// Decoding keeps only the last value for a repeated key, so the secret
			// in the first one would never be scanned — and since nothing was
			// replaced, the original bytes would be handed back as clean.
			name:     "duplicate key hiding a secret is refused",
			doc:      `{"a":"SEC","a":"clean"}`,
			findings: []Finding{{RuleID: "r1", Secret: "SEC"}},
			wantErr:  ErrDuplicateKey,
		},
		{
			name:    "duplicate key nested in the transcript is refused",
			doc:     `{"data":{"raw_session":{"main":[{"content":"SEC","content":"clean"}]}}}`,
			wantErr: ErrDuplicateKey,
		},
		{
			name: "repeating a key in a sibling object is fine",
			doc:  `{"a":{"k":"x"},"b":{"k":"y"},"c":[{"k":1},{"k":2}]}`,
			// Same key name, different objects: nothing is lost, nothing to refuse.
			wantUnchanged: true,
		},
		{
			// Decoder.More reports false for a trailing bracket, so this has to be
			// caught by requiring the input to be exhausted.
			name:    "trailing closing bracket is rejected",
			doc:     `{"a":"b"}]`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "trailing closing brace is rejected",
			doc:     `{"a":"b"}}`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "a second document is rejected",
			doc:     `{"a":"b"}{"c":"d"}`,
			wantErr: ErrInvalidJSON,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := &fakeScanner{findings: tc.findings, requirePresent: !tc.reportAlways}
			r := New(scanner, tc.opts...)

			got, report, err := r.Redact(context.Background(), []byte(tc.doc))

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, report)

			if tc.wantUnchanged {
				assert.Equal(t, tc.doc, string(got), "output must be byte-identical")
				assert.False(t, report.Changed())
				assert.Zero(t, report.Replacements)
			} else {
				assert.True(t, report.Changed())
				assert.Equal(t, tc.wantReplacements, report.Replacements)
				assert.True(t, json.Valid(got), "output must be valid JSON")
			}

			if tc.wantByRule != nil {
				assert.Equal(t, tc.wantByRule, report.ByRule)
			}
			if tc.wantUnlocated != nil {
				assert.Equal(t, tc.wantUnlocated, report.Unlocated)
			}
			for _, s := range tc.mustNotContain {
				assert.NotContains(t, string(got), s)
			}
			for _, s := range tc.mustContain {
				assert.Contains(t, string(got), s)
			}
		})
	}
}

// TestRedactFiltersOnlyMatchingLeaves pins the cost of the path filter. It is
// consulted for the leaves that hold a pending secret, not for every leaf on
// every pass: a transcript has millions of leaves and only a handful of secrets,
// and the AI coding session filter is not cheap.
func TestRedactFiltersOnlyMatchingLeaves(t *testing.T) {
	var consulted []string
	filter := func(p string) bool {
		consulted = append(consulted, p)
		return true
	}

	doc := `{"a":"plain","b":{"c":"x SEC x","d":["plain","plain"]},"e":"plain"}`
	scanner := &fakeScanner{findings: []Finding{{RuleID: "r1", Secret: "SEC"}}, requirePresent: true}

	_, report, err := New(scanner, WithPathFilter(filter)).Redact(context.Background(), []byte(doc))
	require.NoError(t, err)
	require.Equal(t, 1, report.Replacements)
	assert.Equal(t, []string{"/b/c"}, consulted)
}

// recordingScanner reports the findings whose secret is in the scanned text,
// and keeps each text it scanned.
type recordingScanner struct {
	fakeScanner
	texts []string
}

func (r *recordingScanner) Scan(ctx context.Context, text string) ([]Finding, error) {
	r.texts = append(r.texts, text)
	return r.fakeScanner.Scan(ctx, text)
}

func TestRedactOpaqueLeaves(t *testing.T) {
	// The media data is opaque: it is neither scanned nor rewritten.
	opaque := func(_ string, obj map[string]any, key string) bool {
		return key == "data" && obj["type"] == "base64"
	}

	testCases := []struct {
		name string
		doc  string
		want string
		// dataScanned is true when the data is not opaque, so the scanner sees it.
		dataScanned bool
	}{
		{
			name: "secret in opaque data and in text",
			doc:  `{"img":{"type":"base64","data":"aaSECaa"},"t":"y SEC y"}`,
			want: `{"img":{"data":"aaSECaa","type":"base64"},"t":"y [REDACTED:r1] y"}`,
		},
		{
			name: "secret only in opaque data",
			doc:  `{"img":{"type":"base64","data":"aaSECaa"},"t":"plain"}`,
			want: `{"img":{"type":"base64","data":"aaSECaa"},"t":"plain"}`,
		},
		{
			name:        "data that is not opaque is still redacted",
			doc:         `{"img":{"type":"url","data":"aaSECaa"}}`,
			want:        `{"img":{"data":"aa[REDACTED:r1]aa","type":"url"}}`,
			dataScanned: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := &recordingScanner{fakeScanner: fakeScanner{findings: []Finding{{RuleID: "r1", Secret: "SEC"}}, requirePresent: true}}

			got, _, err := New(scanner, WithOpaque(opaque)).Redact(context.Background(), []byte(tc.doc))
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))

			scanned := strings.Contains(strings.Join(scanner.texts, "\n"), "aaSECaa")
			assert.Equal(t, tc.dataScanned, scanned)
		})
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	docs := []string{
		`{"data":{"raw_session":{"main":[{"content":"FAKE-AWS-KEY-NOT-A-REAL-PATTERN"}]}}}`,
		`{"a":"SEC","b":"no secret here"}`,
		`{"a":"line\nSEC","b":["SEC","SEC"]}`,
	}
	findings := []Finding{
		{RuleID: "aws-access-token", Secret: "FAKE-AWS-KEY-NOT-A-REAL-PATTERN"},
		{RuleID: "r1", Secret: "SEC"},
	}

	for _, doc := range docs {
		t.Run(doc, func(t *testing.T) {
			r := New(&fakeScanner{findings: findings, requirePresent: true})

			once, _, err := r.Redact(context.Background(), []byte(doc))
			require.NoError(t, err)
			twice, report, err := r.Redact(context.Background(), once)
			require.NoError(t, err)

			assert.Equal(t, string(once), string(twice))
			assert.False(t, report.Changed(), "a second pass must find nothing")
		})
	}
}

func TestRedactCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := New(&fakeScanner{findings: []Finding{{RuleID: "r1", Secret: "SEC"}}, requirePresent: true})
	got, _, err := r.Redact(ctx, []byte(`{"a":"SEC"}`))

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, got)
}

// RuleIDs reports only the rules that actually redacted something, so it is
// safe to publish as a material annotation.
func TestReportRuleIDs(t *testing.T) {
	r := &Report{ByRule: map[string]int{"b": 2, "a": 1}, Unlocated: map[string]int{"c": 1}}
	assert.Equal(t, []string{"a", "b"}, r.RuleIDs())
	assert.Nil(t, (*Report)(nil).RuleIDs())
}

// TestRedactText covers plain text, which Redact cannot take directly: it
// rewrites the string leaves of a JSON document.
func TestRedactText(t *testing.T) {
	const secret = "s3cr3t-value-0123456789"
	scanner := &fakeScanner{findings: []Finding{{RuleID: "test-token", Secret: secret}}, requirePresent: true}

	testCases := []struct {
		name        string
		text        string
		want        string
		wantChanged bool
	}{
		{name: "a secret is replaced", text: "use " + secret + " now", want: "use [REDACTED:test-token] now", wantChanged: true},
		{name: "text without secrets comes back unchanged", text: "nothing to see", want: "nothing to see"},
		{name: "line breaks and quotes survive", text: "a \"quoted\"\nline " + secret, want: "a \"quoted\"\nline [REDACTED:test-token]", wantChanged: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, report, err := New(scanner).RedactText(context.Background(), tc.text)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantChanged, report.Changed())
		})
	}
}

// TestRedactPlaceholderNeedingEscape checks that a placeholder is inserted in
// its JSON-escaped form. A raw quote or backslash in it would break the string
// the leaf is rebuilt from, and the whole leaf would then be lost to the
// placeholder.
func TestRedactPlaceholderNeedingEscape(t *testing.T) {
	const secret = "s3cr3t-value-0123456789"
	scanner := &fakeScanner{findings: []Finding{{RuleID: "test-token", Secret: secret}}, requirePresent: true}
	placeholder := func(ruleID string) string { return `<"` + ruleID + `"\removed>` }
	matches := func(s string) bool { return strings.HasPrefix(s, `<"`) }

	got, report, err := New(scanner, WithPlaceholder(placeholder, matches)).RedactText(context.Background(), "use "+secret+" now")

	require.NoError(t, err)
	assert.True(t, report.Changed())
	assert.Equal(t, `use <"test-token"\removed> now`, got, "the text around the secret survives")
}
