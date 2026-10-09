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

	"github.com/betterleaks/betterleaks/config"
	"github.com/betterleaks/betterleaks/detect"
	"github.com/betterleaks/betterleaks/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePrivateKey is a private key block the private-key rule matches. Its
// lines are assembled from fragments, like the other synthetic credentials in
// this package.
var fakePrivateKey = "-----BEGIN RSA " + "PRIVATE KEY-----\n" +
	strings.Repeat("MIIEvQIBADANBgkqhkiG9w0BAQEFAASC"+"BKcwggSjAgEAAoIBAQC7\n", 3) +
	"-----END RSA " + "PRIVATE KEY-----"

// fileVisible scans each text as its own file, the way betterleaks scans a
// directory, with the ruleset of the session scanner and allow markers
// honoured. It returns, for each of secrets in order, how many of its occurrences the
// scan does not report: the ones it does not detect and the ones an allow
// marker keeps.
func fileVisible(t *testing.T, texts, secrets []string) []int {
	t.Helper()
	useRE2()
	cfg, err := config.ParseTOMLString(rulesConfig, "")
	require.NoError(t, err)
	d := detect.NewDetectorContext(context.Background(), cfg, detect.ValidationOptions{})
	d.Redact = 0
	d.MaxDecodeDepth = 0
	d.SkipFindingAppend = true

	visible := make([]int, len(secrets))
	for _, text := range texts {
		// An occurrence is identified by its line, so that a secret two rules
		// report on the same line counts once.
		reported := map[string]map[int]struct{}{}
		report := func(secret string, line int) {
			if reported[secret] == nil {
				reported[secret] = map[int]struct{}{}
			}
			reported[secret][line] = struct{}{}
		}
		for r := range d.Run(context.Background(), fileSource{text: text}) {
			require.NoError(t, r.Err)
			report(r.Finding.Secret, r.Finding.StartLine)
			for _, set := range r.Finding.ComponentSets {
				for _, c := range set.Components {
					if c != nil && !c.Optional {
						report(c.Secret, c.StartLine)
					}
				}
			}
		}
		for i, s := range secrets {
			visible[i] += strings.Count(text, s) - len(reported[s])
		}
	}
	return visible
}

// fileSource is a single file's content.
type fileSource struct{ text string }

func (s fileSource) Fragments(_ context.Context, yield sources.FragmentsFunc) error {
	return yield(sources.Fragment{Raw: s.text}, nil)
}

// sessionVisible redacts a session document whose string leaves are texts, the
// way an AI coding session is redacted, and returns how many occurrences of
// each of secrets, in order, are left in it.
func sessionVisible(t *testing.T, texts, secrets []string) []int {
	t.Helper()
	scanner, err := DefaultScanner()
	require.NoError(t, err)

	doc, err := json.Marshal(map[string][]string{"content": texts})
	require.NoError(t, err)

	out, _, err := New(scanner, WithAllowMarkers()).Redact(context.Background(), doc)
	require.NoError(t, err)

	var got map[string][]string
	require.NoError(t, json.Unmarshal(out, &got))
	visible := make([]int, len(secrets))
	for _, leaf := range got["content"] {
		for i, s := range secrets {
			visible[i] += strings.Count(leaf, s)
		}
	}
	return visible
}

// TestSessionRedactionMatchesFileScan compares the real detector over files
// with the session redaction of the same text as string leaves. The ruleset is
// the same on both sides, so the comparison is about where the text is: a file
// has real line breaks, a session leaf is a JSON string where a line break is
// the escape \n and a quote is \".
//
// Each case gives, per secret, the occurrences left visible. A case with a
// difference documents why the session does not do what a file scan does.
func TestSessionRedactionMatchesFileScan(t *testing.T) {
	testCases := []struct {
		name string
		// texts are the files of the file scan, and the leaves of the session.
		texts   []string
		secrets []string
		// wantFile and wantSession are the occurrences of each of secrets left
		// visible, in the same order.
		wantFile    []int
		wantSession []int
		// difference explains why wantSession is not wantFile.
		difference string
	}{
		{
			name:     "a secret without a marker is reported",
			texts:    []string{"export GITHUB_TOKEN=" + fakeGitHubPAT + "\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{0},
		},
		{
			name:     "a marker on the line of the secret keeps it",
			texts:    []string{"const pat = '" + fakeGitHubPAT + "' // gitleaks:allow\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{1},
		},
		{
			name:     "the betterleaks marker keeps it too",
			texts:    []string{"token: " + fakeGitHubPAT + " # betterleaks:allow\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{1},
		},
		{
			name:     "a marker on the next line does not keep it",
			texts:    []string{"const pat = '" + fakeGitHubPAT + "'\n// gitleaks:allow\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{0},
		},
		{
			name:     "a marker on the previous line does not keep it",
			texts:    []string{"// gitleaks:allow\nconst pat = '" + fakeGitHubPAT + "'\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{0},
		},
		{
			name:     "a marker keeps only the occurrence on its line",
			texts:    []string{"a = '" + fakeGitHubPAT + "' # gitleaks:allow\nexport GITHUB_TOKEN=" + fakeGitHubPAT + "\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{1},
		},
		{
			name:     "a marker in another file or leaf does not keep it",
			texts:    []string{"export GITHUB_TOKEN=" + fakeGitHubPAT + "\n", "x // gitleaks:allow\n"},
			secrets:  []string{fakeGitHubPAT},
			wantFile: []int{0},
		},
		{
			name:     "an unquoted password",
			texts:    []string{"password: " + samplePassword + "\n"},
			secrets:  []string{samplePassword},
			wantFile: []int{0},
		},
		{
			name:     "a single-quoted key",
			texts:    []string{"api_key = '" + sampleAPIKey + "'\n"},
			secrets:  []string{sampleAPIKey},
			wantFile: []int{0},
		},
		{
			name:     "an AWS key pair",
			texts:    []string{"AWS_ACCESS_KEY_ID=" + fakeAWSKey + "\nAWS_SECRET_ACCESS_KEY=" + fakeAWSSecret + "\n"},
			secrets:  []string{fakeAWSKey, fakeAWSSecret},
			wantFile: []int{0, 0},
		},
		{
			// The file scan drops the whole pair, then reports the secret key with
			// the generic rule. The session keeps only the key id. Either way
			// only the id is left.
			name:     "a marker on the AWS key id line keeps the id only",
			texts:    []string{"AWS_ACCESS_KEY_ID=" + fakeAWSKey + " # gitleaks:allow\nAWS_SECRET_ACCESS_KEY=" + fakeAWSSecret + "\n"},
			secrets:  []string{fakeAWSKey, fakeAWSSecret},
			wantFile: []int{1, 0},
		},
		{
			name:     "a private key",
			texts:    []string{fakePrivateKey + "\n"},
			secrets:  []string{fakePrivateKey},
			wantFile: []int{0},
		},
		{
			name:        "a marker on the last line of a private key",
			texts:       []string{fakePrivateKey + " # gitleaks:allow\n"},
			secrets:     []string{fakePrivateKey},
			wantFile:    []int{1},
			wantSession: []int{0},
			difference: "betterleaks checks the marker against every line of a match, so one marker " +
				"keeps a whole multi-line secret; the session redacts a secret that spans lines",
		},
		{
			name:        "a double-quoted key",
			texts:       []string{"const api_key = \"" + sampleAPIKey + "\"\n"},
			secrets:     []string{sampleAPIKey},
			wantFile:    []int{0},
			wantSession: []int{1},
			difference:  "known gap: the session scans JSON, where the quote is \\\" and the generic rules do not allow a backslash before the value",
		},
		{
			name:        "a double-quoted password",
			texts:       []string{"password = \"" + samplePassword + "\"\n"},
			secrets:     []string{samplePassword},
			wantFile:    []int{0},
			wantSession: []int{1},
			difference:  "known gap: the session scans JSON, where the quote is \\\"",
		},
		{
			name:        "a value on the line after its key",
			texts:       []string{"api_key:\n  " + sampleAPIKey + "\n"},
			secrets:     []string{sampleAPIKey},
			wantFile:    []int{0},
			wantSession: []int{1},
			difference:  "known gap: in JSON the line break is the escape \\n, which the rules do not read as whitespace",
		},
		{
			name:        "an import on another line of the text",
			texts:       []string{"import { a } from 'b'\napi_key = " + sampleAPIKey + "\n"},
			secrets:     []string{sampleAPIKey},
			wantFile:    []int{0},
			wantSession: []int{1},
			difference: "known gap: the generic-api-key rule drops a finding whose line is an import, " +
				"and a whole session leaf is one line",
		},
		{
			name:        "an AWS key id at the start of a line",
			texts:       []string{"x\n" + fakeAWSKey + "\naws_secret = " + fakeAWSSecret + "\n"},
			secrets:     []string{fakeAWSKey, fakeAWSSecret},
			wantFile:    []int{0, 0},
			wantSession: []int{1, 0},
			difference:  "known gap: after the escape \\n the id follows the letter n, so the rule's word boundary does not match",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wantSession := tc.wantSession
			if tc.difference == "" {
				wantSession = tc.wantFile
			} else {
				require.NotEqual(t, tc.wantFile, wantSession, "a case with a difference expects one")
			}

			// Compared as counts by position, so that a failure does not print a
			// secret.
			assert.Equal(t, tc.wantFile, fileVisible(t, tc.texts, tc.secrets), "file scan")
			assert.Equal(t, wantSession, sessionVisible(t, tc.texts, tc.secrets), "session")
		})
	}
}
