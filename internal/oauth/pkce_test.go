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

package oauth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCE(t *testing.T) {
	verifier, err := NewCodeVerifier()
	require.NoError(t, err)
	assert.Len(t, verifier, 43)

	other, err := NewCodeVerifier()
	require.NoError(t, err)
	assert.NotEqual(t, verifier, other)

	challenge := CodeChallengeS256(verifier)
	assert.True(t, ValidS256Challenge(challenge))

	testCases := []struct {
		name      string
		verifier  string
		challenge string
		want      bool
	}{
		{"matching verifier", verifier, challenge, true},
		{"other verifier", other, challenge, false},
		{"empty verifier", "", CodeChallengeS256(""), false},
		{"verifier too short", "abc", CodeChallengeS256("abc"), false},
		{"verifier too long", strings.Repeat("a", 129), CodeChallengeS256(strings.Repeat("a", 129)), false},
		{"verifier with invalid characters", verifier[:42] + "/", CodeChallengeS256(verifier[:42] + "/"), false},
		{"plain challenge", verifier, verifier, false},
		{"empty challenge", verifier, "", false},
		// RFC 7636 appendix B
		{"rfc example", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, VerifyS256(tc.verifier, tc.challenge))
		})
	}
}

func TestValidS256Challenge(t *testing.T) {
	testCases := []struct {
		challenge string
		want      bool
	}{
		{"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", true},
		{"", false},
		{"short", false},
		{"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM=", false},
		{"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw+cM", false},
	}

	for _, tc := range testCases {
		t.Run(tc.challenge, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidS256Challenge(tc.challenge))
		})
	}
}
