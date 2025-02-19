//
// Copyright 2025 The Chainloop Authors.
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
	"os"
	"testing"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		expectErr bool
	}{
		{
			name:   "invalid bundle according to spec, but still verifiable",
			roots:  roots,
			bundle: "testdata/bundle_wrongversion.json",
		},
		{
			name:   "valid bundle, but sig encoded twice",
			roots:  roots,
			bundle: "testdata/bundle_valid_pre1832.json",
		},
		{
			name:   "valid bundle",
			roots:  roots,
			bundle: "testdata/bundle_valid.json",
		},
		{
			name:      "corrupted bundle",
			roots:     roots,
			bundle:    "testdata/bundle_invalid.json",
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundleBytes, err := os.ReadFile(tc.bundle)
			require.NoError(t, err)
			err = VerifyBundle(context.TODO(), bundleBytes, tc.roots)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
