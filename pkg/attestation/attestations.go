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

package attestation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigstoredsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	"google.golang.org/protobuf/encoding/protojson"
)

// JSONEnvelopeWithDigest returns the JSON content of the envelope and its digest.
func JSONEnvelopeWithDigest(envelope *dsse.Envelope) ([]byte, cr_v1.Hash, error) {
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("marshaling the envelope: %w", err)
	}

	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("calculating the digest: %w", err)
	}

	return jsonContent, h, nil
}

// DSSEEnvelopeFromBundle Extracts a DSSE envelope from a Sigstore bundle (Sigstore bundles have their own protobuf implementation for DSSE)
func DSSEEnvelopeFromBundle(bundle *protobundle.Bundle) *dsse.Envelope {
	sigstoreEnvelope := bundle.GetDsseEnvelope()
	sig := sigstoreEnvelope.GetSignatures()[0].GetSig()
	// See bug: https://github.com/chainloop-dev/chainloop/issues/1832
	// signature might be encoded twice. Let's try to fix it first.
	// TODO: remove this once the bug is fixed
	sigBytes := sig
	dst := make([]byte, base64.RawURLEncoding.DecodedLen(len(sig)))
	i, err := base64.StdEncoding.Decode(dst, sig)
	if err == nil {
		// it was already encoded, let's use the decoded one
		sigBytes = dst[:i]
	}
	return &dsse.Envelope{
		PayloadType: sigstoreEnvelope.PayloadType,
		Payload:     base64.StdEncoding.EncodeToString(sigstoreEnvelope.Payload),
		Signatures: []dsse.Signature{
			{
				KeyID: sigstoreEnvelope.GetSignatures()[0].GetKeyid(),
				Sig:   base64.StdEncoding.EncodeToString(sigBytes),
			},
		},
	}
}

func BundleFromDSSEEnvelope(dsseEnvelope *dsse.Envelope) (*protobundle.Bundle, error) {
	// DSSE Envelope is already base64 encoded, we need to decode to prevent it from being encoded twice
	payload, err := base64.StdEncoding.DecodeString(dsseEnvelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("decoding: %w", err)
	}
	return &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.3",
		Content: &protobundle.Bundle_DsseEnvelope{DsseEnvelope: &sigstoredsse.Envelope{
			Payload:     payload,
			PayloadType: dsseEnvelope.PayloadType,
			Signatures: []*sigstoredsse.Signature{
				{
					Sig:   []byte(dsseEnvelope.Signatures[0].Sig),
					Keyid: dsseEnvelope.Signatures[0].KeyID,
				},
			},
		}},
		VerificationMaterial: &protobundle.VerificationMaterial{},
	}, nil
}

func DSSEEnvelopeFromRaw(bundle, envelope []byte) (*dsse.Envelope, error) {
	var dsseEnv dsse.Envelope
	if bundle != nil {
		var attBundle protobundle.Bundle
		if err := protojson.Unmarshal(bundle, &attBundle); err != nil {
			return nil, fmt.Errorf("unmarshalling bundle: %w", err)
		}
		dsseEnv = *DSSEEnvelopeFromBundle(&attBundle)
	} else {
		if err := json.Unmarshal(envelope, &dsseEnv); err != nil {
			return nil, fmt.Errorf("unmarshalling envelope: %w", err)
		}
	}
	return &dsseEnv, nil
}

// TODO: remove this fix once `AttestationServiceStoreRequest.Bundle` is fully removed,
//	     and move the logic to BundleFromDSSEEnvelope method instead (where the bug is originated)

// FixSignatureInBundle removes any additional base64 encoding from the signature in the bundle.
// Old attestations have signatures base64 encoded twice, see https://github.com/chainloop-dev/chainloop/issues/1832
func FixSignatureInBundle(bundle *protobundle.Bundle) {
	sig := bundle.GetDsseEnvelope().GetSignatures()[0].GetSig()
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(sig)))
	i, err := base64.StdEncoding.Decode(dst, sig)
	if err == nil {
		// it was encoded twice. Use it
		sig = dst[:i]
	}
	bundle.GetDsseEnvelope().GetSignatures()[0].Sig = sig
}
