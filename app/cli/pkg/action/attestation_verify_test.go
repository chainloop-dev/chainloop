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

package action

import (
	"errors"
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/verifier"
	"github.com/stretchr/testify/assert"
)

func TestVerificationFailedFatally(t *testing.T) {
	cases := []struct {
		name string
		err  error
		// expect reports whether the error must abort the command instead of
		// leaving the attestation reported as unverified.
		expect bool
	}{
		{
			name:   "no verification material",
			err:    verifier.ErrMissingVerificationMaterial,
			expect: false,
		},
		{
			// A control plane with no timestamp authorities configured cannot
			// tell us whether the timestamp is good. That is not the
			// attestation's fault and must not mask the evidence.
			name:   "control plane has no TSA roots configured",
			err:    fmt.Errorf("could not verify timestamps: %w", verifier.ErrNoTSARootsConfigured),
			expect: false,
		},
		{
			name:   "timestamp signed by an authority we do not have",
			err:    fmt.Errorf("could not verify timestamps: %w", verifier.ErrTSASignerNotTrusted),
			expect: false,
		},
		{
			name:   "timestamp does not verify against a trusted authority",
			err:    fmt.Errorf("could not verify timestamps: %w", verifier.ErrTSAResponseInvalid),
			expect: true,
		},
		{
			name:   "signing certificate not valid at timestamp time",
			err:    fmt.Errorf("could not verify timestamps: %w", verifier.ErrSigningCertNotValidAtTimestamp),
			expect: true,
		},
		{
			name:   "invalid signature",
			err:    errors.New("validating the DSSE envelope: signature not verified"),
			expect: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, verificationFailedFatally(tc.err))
		})
	}
}
