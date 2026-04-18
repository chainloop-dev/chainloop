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

package attestation_test

import (
	"encoding/base64"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Guards against the double-base64 bug in https://github.com/chainloop-dev/chainloop/issues/1832.
func TestBundleFromDSSEEnvelopeDecodesSignature(t *testing.T) {
	rawSig := []byte{0x30, 0x44, 0x02, 0x20, 0xAA, 0xBB, 0xCC, 0xDD}
	rawPayload := []byte(`{"_type":"statement"}`)

	env := &dsse.Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.StdEncoding.EncodeToString(rawPayload),
		Signatures: []dsse.Signature{
			{KeyID: "key-1", Sig: base64.StdEncoding.EncodeToString(rawSig)},
		},
	}

	bundle, err := attestation.BundleFromDSSEEnvelope(env)
	require.NoError(t, err)

	gotEnv := bundle.GetDsseEnvelope()
	assert.Equal(t, rawPayload, gotEnv.GetPayload())
	require.Len(t, gotEnv.GetSignatures(), 1)
	assert.Equal(t, rawSig, gotEnv.GetSignatures()[0].GetSig())
	assert.Equal(t, "key-1", gotEnv.GetSignatures()[0].GetKeyid())
}

func TestBundleRoundTripWithFixedSignature(t *testing.T) {
	rawSig := []byte{0x30, 0x44, 0x02, 0x20, 0xAA, 0xBB, 0xCC, 0xDD}
	encodedSig := base64.StdEncoding.EncodeToString(rawSig)

	env := &dsse.Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.StdEncoding.EncodeToString([]byte("payload")),
		Signatures: []dsse.Signature{
			{KeyID: "key-1", Sig: encodedSig},
		},
	}

	bundle, err := attestation.BundleFromDSSEEnvelope(env)
	require.NoError(t, err)

	before := bundle.GetDsseEnvelope().GetSignatures()[0].GetSig()
	attestation.FixSignatureInBundle(bundle)
	assert.Equal(t, before, bundle.GetDsseEnvelope().GetSignatures()[0].GetSig(),
		"FixSignatureInBundle should be a no-op on properly formed bundles")

	gotEnv := attestation.DSSEEnvelopeFromBundle(bundle)
	assert.Equal(t, encodedSig, gotEnv.Signatures[0].Sig)
}

func TestFixSignatureInBundleRepairsLegacyBundles(t *testing.T) {
	rawSig := []byte{0x30, 0x44, 0x02, 0x20, 0xAA, 0xBB, 0xCC, 0xDD}
	encodedSig := base64.StdEncoding.EncodeToString(rawSig)

	env := &dsse.Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.StdEncoding.EncodeToString([]byte("payload")),
		Signatures: []dsse.Signature{
			{KeyID: "key-1", Sig: encodedSig},
		},
	}
	bundle, err := attestation.BundleFromDSSEEnvelope(env)
	require.NoError(t, err)

	// Simulate the legacy bug: signature is stored as the ASCII bytes of the base64 string.
	bundle.GetDsseEnvelope().GetSignatures()[0].Sig = []byte(encodedSig)

	attestation.FixSignatureInBundle(bundle)
	assert.Equal(t, rawSig, bundle.GetDsseEnvelope().GetSignatures()[0].GetSig())
}
