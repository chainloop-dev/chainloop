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

package orcarouter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderDefinition(t *testing.T) {
	p := Default()
	assert.Equal(t, ID, p.ID)
	assert.Equal(t, "OrcaRouter Cyber Inference", p.Name)
	assert.Equal(t, "https://api.orcarouter.ai/v1", p.BaseURL)
	assert.Equal(t, "ORCAROUTER_API_KEY", p.EnvKey)
	assert.Equal(t, "orcarouter/auto", p.DefaultModel)
	// The brand name and the orcarouter keyword stay visible.
	assert.Contains(t, strings.ToLower(p.Name), "orcarouter")
	assert.NotContains(t, p.BaseURL, "/auth")
}

func TestRecommendedModels(t *testing.T) {
	// The default and the fallback are OrcaRouter's own routing models; the
	// cyber models keep the namespace the live catalog publishes.
	assert.Equal(t, []string{"orcarouter/auto", "orca/orcacyber-ultra-1.0", "orca/orcacyber-zero-1.5", "orcarouter/fusion"}, RecommendedModels())
	for _, id := range RecommendedModels() {
		assert.True(t, strings.HasPrefix(id, "orcarouter/") || strings.HasPrefix(id, "orca/orcacyber-"), id)
	}
	assert.NotContains(t, RecommendedModels(), "deepseek/deepseek-v4.1-flash", "a verification-only model is never recommended to users")
}

func TestMaskKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{name: "empty", key: "", want: ""},
		{name: "short", key: "abc", want: "***"},
		{name: "typical", key: "sk-orca-0123456789abcdef", want: "sk-o****************cdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskKey(tc.key)
			assert.Equal(t, tc.want, got)
			if tc.key != "" {
				assert.NotEqual(t, tc.key, got)
				assert.NotContains(t, got, "0123456789")
			}
		})
	}
}

func TestAPIKeyAdapter(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		wantErr bool
		source  Source
	}{
		{name: "valid", key: "sk-orca-abcdef123456", source: SourceAPIKey},
		{name: "surrounding whitespace", key: "  sk-orca-abcdef123456\n", source: SourceAPIKey},
		{name: "empty", key: "", wantErr: true},
		{name: "wrong prefix", key: "sk-ant-abcdef", wantErr: true},
		{name: "prefix only", key: "sk-orca-", wantErr: true},
		{name: "embedded space", key: "sk-orca-ab cd", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cred, err := (APIKeyAdapter{}).Acquire(tc.key)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidKey)
				assert.Empty(t, cred.Key)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.source, cred.Source)
			assert.Equal(t, strings.TrimSpace(tc.key), cred.Key)
		})
	}
}

func TestCredentialRedacted(t *testing.T) {
	cred := Credential{Key: "sk-orca-secret-value-here", Source: SourcePKCE, UserID: "u1", Scope: "api"}
	red := cred.Redacted()
	assert.NotEqual(t, cred.Key, red.Key)
	assert.NotContains(t, red.Key, "secret-value")
	assert.Equal(t, cred.UserID, red.UserID)
	assert.Equal(t, cred.Source, red.Source)
}
