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

package commitverification

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationStatusText(t *testing.T) {
	testCases := []struct {
		status VerificationStatus
		text   string
	}{
		{VerificationStatusUnspecified, "unspecified"},
		{VerificationStatusVerified, "verified"},
		{VerificationStatusUnverified, "unverified"},
		{VerificationStatusUnavailable, "unavailable"},
		{VerificationStatusNotApplicable, "not_applicable"},
	}

	for _, tc := range testCases {
		t.Run(tc.text, func(t *testing.T) {
			got, err := tc.status.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tc.text, string(got))
			assert.Equal(t, tc.text, tc.status.String())

			var parsed VerificationStatus
			require.NoError(t, parsed.UnmarshalText([]byte(tc.text)))
			assert.Equal(t, tc.status, parsed)
		})
	}
}

func TestVerificationStatusTextErrors(t *testing.T) {
	_, err := VerificationStatus(99).MarshalText()
	require.Error(t, err)

	var parsed VerificationStatus
	require.Error(t, parsed.UnmarshalText([]byte("trusted")))
}

const signatureAlgorithmPGP = "PGP"

func TestCommitVerificationJSON(t *testing.T) {
	want := &CommitVerification{
		Attempted:          true,
		Status:             VerificationStatusVerified,
		Reason:             "Commit signed and verified",
		Platform:           "gitlab",
		KeyID:              "1",
		SignatureAlgorithm: signatureAlgorithmPGP,
	}

	raw, err := json.Marshal(want)
	require.NoError(t, err)
	assert.JSONEq(t, `{"attempted":true,"status":"verified","reason":"Commit signed and verified","platform":"gitlab","key_id":"1","signature_algorithm":"PGP"}`, string(raw))

	var got CommitVerification
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, want, &got)
}
