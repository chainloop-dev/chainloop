//
// Copyright 2025-2026 The Chainloop Authors.
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

package verifier

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"os"
	"testing"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestVerifyBundle(t *testing.T) {
	ca, err := os.ReadFile("testdata/ca.pub")
	require.NoError(t, err)
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(ca))
	require.NoError(t, err)
	roots := &TrustedRoot{Keys: map[string][]*x509.Certificate{
		"2a522d9652e0933d2a1237c395bc116e012f86dffff13122da59f76e0d2abe27": certs,
	}}

	cases := []struct {
		name      string
		roots     *TrustedRoot
		bundle    string
		expectErr string
		// expectSentinel, when set, is asserted with errors.Is in addition to
		// (or instead of) the substring match.
		expectSentinel error
	}{
		{
			name:   "invalid bundle, but still verifiable",
			roots:  roots,
			bundle: "testdata/bundle_wrongversion.json",
		},
		{
			name:   "valid bundle",
			roots:  roots,
			bundle: "testdata/bundle_valid.json",
		},
		{
			name:           "valid bundle without verification material",
			roots:          roots,
			bundle:         "testdata/bundle_valid_nomaterial.json",
			expectErr:      "missing material",
			expectSentinel: ErrMissingVerificationMaterial,
		},
		{
			name:      "corrupted bundle",
			roots:     roots,
			bundle:    "testdata/bundle_invalid.json",
			expectErr: "validating the DSSE envelope",
		},
		{
			name:      "legacy DSSE envelope (not a bundle)",
			roots:     roots,
			bundle:    "testdata/dsse_envelope.json",
			expectErr: "invalid bundle",
		},
		{
			// a cert-less bundle carrying only a timestamp must never be reported as verified.
			// It is rejected at the mandatory-signature gate before timestamp validation runs,
			// so the timestamp can never be the deciding factor.
			name:           "timestamp-only bundle (no signing key) is rejected",
			roots:          roots,
			bundle:         "testdata/bundle_with_bad_timestamp.json",
			expectErr:      "missing material",
			expectSentinel: ErrMissingVerificationMaterial,
		},
		{
			// public-key bundles have no trusted key
			// set to verify against and must fail rather than fall through to
			// the timestamp-only path.
			name:           "public key bundle is rejected as unsupported",
			roots:          roots,
			bundle:         "testdata/bundle_with_publickey.json",
			expectErr:      "unsupported verification material",
			expectSentinel: ErrUnsupportedVerificationMaterial,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundleBytes, err := os.ReadFile(tc.bundle)
			require.NoError(t, err)
			err = VerifyBundle(context.TODO(), bundleBytes, tc.roots)
			if tc.expectErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
				if tc.expectSentinel != nil {
					assert.True(t, errors.Is(err, tc.expectSentinel),
						"expected %v, got: %v", tc.expectSentinel, err)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestVerifyTimestamps_TypedErrors(t *testing.T) {
	ca, err := os.ReadFile("testdata/ca.pub")
	require.NoError(t, err)
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(ca))
	require.NoError(t, err)

	cases := []struct {
		name           string
		roots          *TrustedRoot
		expectSentinel error
	}{
		{
			name: "bad timestamp with TSA configured",
			roots: &TrustedRoot{
				TimestampAuthorities: map[string][]*x509.Certificate{
					"fake-tsa": certs,
				},
			},
			expectSentinel: ErrTSAResponseInvalid,
		},
		{
			name:           "timestamp with no TSA roots configured",
			roots:          &TrustedRoot{},
			expectSentinel: ErrNoTSARootsConfigured,
		},
	}

	bundleBytes, err := os.ReadFile("testdata/bundle_with_bad_timestamp.json")
	require.NoError(t, err)

	// VerifyTimestamps is exercised directly: the bad-timestamp fixture is a
	// cert-less bundle, which VerifyBundle now rejects at the mandatory-signature
	// gate before timestamp validation would run.
	bundle := new(protobundle.Bundle)
	require.NoError(t, protojson.Unmarshal(bundleBytes, bundle))
	sb := &sigstorebundle.Bundle{Bundle: bundle}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTimestamps(sb, tc.roots)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.expectSentinel),
				"expected %v, got: %v", tc.expectSentinel, err)
		})
	}
}
