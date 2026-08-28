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

package prinfo_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/prinfo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The evidence advertises a schema id, and that id must point at a schema this
// package actually ships. Consumers rely on it to pick the right validator.
func TestEvidenceSchemaURLMatchesLatestVersion(t *testing.T) {
	assert.Equal(t, prinfo.SchemaURL(prinfo.LatestVersion), prinfo.EvidenceSchemaURL)

	raw, err := prinfo.Schema(prinfo.LatestVersion)
	require.NoError(t, err)

	var doc struct {
		ID string `json:"$id"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &doc))
	assert.Equal(t, prinfo.EvidenceSchemaURL, doc.ID)
}

func TestSchema(t *testing.T) {
	for _, version := range prinfo.Versions() {
		t.Run(string(version), func(t *testing.T) {
			raw, err := prinfo.Schema(version)
			require.NoError(t, err)
			assert.NotEmpty(t, raw)

			var doc struct {
				ID string `json:"$id"`
			}
			require.NoError(t, json.Unmarshal([]byte(raw), &doc))
			assert.Equal(t, prinfo.SchemaURL(version), doc.ID)
		})
	}

	t.Run("unknown version", func(t *testing.T) {
		_, err := prinfo.Schema("9.9")
		assert.ErrorContains(t, err, `invalid PR info schema version "9.9"`)
	})
}

// Published schemas are immutable: 1.0-1.2 take the author as a plain string and only
// 1.3 introduced the object form (keeping the string one for backwards compatibility).
// A stale //go:generate directive once rewrote the published 1.2 schema from a later
// revision of Data, making it reject the very payloads it had been published to accept.
func TestPublishedAuthorShapeIsStable(t *testing.T) {
	const stringAuthor = `"octocat"`
	const objectAuthor = `{"login": "octocat", "type": "User"}`

	testCases := []struct {
		version    prinfo.Version
		author     string
		wantErr    bool
		authorForm string
	}{
		{version: prinfo.Version1_0, author: stringAuthor, authorForm: "string"},
		{version: prinfo.Version1_0, author: objectAuthor, authorForm: "object", wantErr: true},
		{version: prinfo.Version1_1, author: stringAuthor, authorForm: "string"},
		{version: prinfo.Version1_1, author: objectAuthor, authorForm: "object", wantErr: true},
		{version: prinfo.Version1_2, author: stringAuthor, authorForm: "string"},
		{version: prinfo.Version1_2, author: objectAuthor, authorForm: "object", wantErr: true},
		{version: prinfo.Version1_3, author: stringAuthor, authorForm: "string"},
		{version: prinfo.Version1_3, author: objectAuthor, authorForm: "object"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.version)+"/"+tc.authorForm, func(t *testing.T) {
			payload := `{
				"platform": "github",
				"type": "pull_request",
				"number": "123",
				"url": "https://github.com/owner/repo/pull/123",
				"author": ` + tc.author + `
			}`

			var data any
			require.NoError(t, json.Unmarshal([]byte(payload), &data))

			err := prinfo.Validate(data, tc.version)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNewEvidence(t *testing.T) {
	evidence := prinfo.NewEvidence(prinfo.Data{
		Platform: "github",
		Type:     "pull_request",
		Number:   "1",
		URL:      "https://github.com/owner/repo/pull/1",
	})

	assert.Equal(t, prinfo.EvidenceID, evidence.ID)
	assert.Equal(t, prinfo.EvidenceSchemaURL, evidence.Schema)

	// the payload it produces must validate against the schema it declares
	raw, err := json.Marshal(evidence.Data)
	require.NoError(t, err)

	var data any
	require.NoError(t, json.Unmarshal(raw, &data))
	require.NoError(t, prinfo.Validate(data, prinfo.LatestVersion))
}

func TestValidateFromFile(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "valid PR info with all fields",
			filePath: "./testdata/pr_info_valid.json",
		},
		{
			name:     "missing required fields",
			filePath: "./testdata/pr_info_missing_required.json",
			wantErr:  "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v any
			require.NoError(t, json.Unmarshal(f, &v))

			// an empty version falls back to the latest one
			err = prinfo.Validate(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateErrors(t *testing.T) {
	testCases := []struct {
		name    string
		data    any
		version prinfo.Version
		wantErr string
	}{
		{
			name:    "completely wrong format",
			data:    map[string]any{"spdxVersion": "SPDX-2.3"},
			version: prinfo.LatestVersion,
			wantErr: "missing properties",
		},
		{
			name:    "unknown version",
			data:    map[string]any{},
			version: "9.9",
			wantErr: `invalid PR info schema version "9.9"`,
		},
		{
			name:    "payload not decoded from JSON",
			data:    struct{ Platform string }{Platform: "github"},
			version: prinfo.LatestVersion,
			wantErr: prinfo.ErrInvalidJSONPayload.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, prinfo.Validate(tc.data, tc.version), tc.wantErr)
		})
	}
}
