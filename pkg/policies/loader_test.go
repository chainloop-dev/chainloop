//
// Copyright 2024 The Chainloop Authors.
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

package policies

import (
	"testing"

	v12 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureScheme(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected []string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "we want ta file scheme and received one",
			ref:      "file:///tmp/policy.json",
			expected: []string{fileScheme},
			wantPath: "/tmp/policy.json",
		},
		{
			name:     "we want a file scheme and received a chainloop scheme",
			ref:      "chainloop:///policy.json",
			expected: []string{fileScheme},
			wantErr:  true,
		},
		{
			name:     "we want a https scheme",
			ref:      "https://example.com/policy.json",
			expected: []string{httpsScheme},
			wantPath: "example.com/policy.json",
		},
		{
			name:     "it works with both http and https",
			ref:      "http://example.com/policy.json",
			expected: []string{httpsScheme, httpScheme},
			wantPath: "example.com/policy.json",
		},
		{
			name:     "doest not supports default schema",
			ref:      "built-in-policy",
			expected: []string{chainloopScheme},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScheme, err := ensureScheme(tt.ref, tt.expected...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, gotScheme)
		})
	}
}

func TestPolicyReferenceResourceDescriptor(t *testing.T) {
	testCases := []struct {
		ref    string
		digest crv1.Hash
		want   *v12.ResourceDescriptor
	}{
		{
			ref: "chainloop:///policy.json",
			digest: crv1.Hash{
				Algorithm: "sha256",
				Hex:       "1234",
			},
			want: &v12.ResourceDescriptor{
				Name: "chainloop:///policy.json",
				Digest: map[string]string{
					"sha256": "1234",
				},
			},
		},
	}

	for _, tc := range testCases {
		got := policyReferenceResourceDescriptor(tc.ref, tc.digest)
		assert.Equal(t, tc.want, got)
	}
}

func TestExtractNameAndDigestFromRef(t *testing.T) {
	testCases := []struct {
		ref  string
		want []string
	}{
		{
			ref:  "chainloop://policy.json@sha256:1234",
			want: []string{"chainloop://policy.json", "sha256:1234"},
		},
		{
			ref:  "chainloop://policy.json",
			want: []string{"chainloop://policy.json", ""},
		},
		{
			ref:  "",
			want: []string{"", ""},
		},
	}

	for _, tc := range testCases {
		gotName, gotDigest := ExtractDigest(tc.ref)
		assert.Equal(t, tc.want[0], gotName)
		assert.Equal(t, tc.want[1], gotDigest)
	}
}
