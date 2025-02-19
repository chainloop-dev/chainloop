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
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/sigstore/timestamp-authority/pkg/verification"
	"google.golang.org/protobuf/encoding/protojson"
)

type TrustedRoot struct {
	// map key identifiers to a chain of certificates
	Keys map[string][]*x509.Certificate
}

var ErrMissingVerificationMaterial = errors.New("missing material")

func VerifyBundle(ctx context.Context, bundleBytes []byte, tr *TrustedRoot) error {
	if bundleBytes == nil {
		return ErrMissingVerificationMaterial
	}

	bundle := new(protobundle.Bundle)
	// unmarshal and validate
	if err := protojson.Unmarshal(bundleBytes, bundle); err != nil {
		return fmt.Errorf("invalid bundle: %w", err)
	}

	// fix for old attestations
	fixSignatureInBundle(bundle)

	var signingCert *x509.Certificate
	if bundle.GetVerificationMaterial() == nil || bundle.GetVerificationMaterial().GetCertificate() == nil {
		// it's a malformed bundle (according to specs) but still verifiable
		// TODO: get rid of this compatibility logic in a future release
		if bundle.GetVerificationMaterial().GetX509CertificateChain() != nil {
			certs := bundle.GetVerificationMaterial().GetX509CertificateChain().GetCertificates()
			if len(certs) == 0 {
				return ErrMissingVerificationMaterial
			}
			rawBytes := certs[0].GetRawBytes()
			if len(rawBytes) == 0 {
				return ErrMissingVerificationMaterial
			}

			var err error
			signingCert, err = x509.ParseCertificate(rawBytes)
			if err != nil {
				return fmt.Errorf("invalid certificate: %w", err)
			}
		} else {
			// nothing to verify
			return ErrMissingVerificationMaterial
		}
	}

	// Use sigstore helpers to validate and extract materials
	if signingCert == nil {
		sb := &sigstorebundle.Bundle{Bundle: bundle}
		//if err := sb.UnmarshalJSON(bundleBytes); err != nil {
		//	return fmt.Errorf("invalid bundle: %w", err)
		//}

		vc, err := sb.VerificationContent()
		if err != nil {
			return fmt.Errorf("could not get verification material: %w", err)
		}
		signingCert = vc.GetCertificate()
		if signingCert == nil {
			return ErrMissingVerificationMaterial
		}

		sc, err := sb.SignatureContent()
		if err != nil {
			return fmt.Errorf("could not get signature material: %w", err)
		}

		signedTimestamps, err := sb.Timestamps()
		if err != nil {
			return fmt.Errorf("could not get timestamps from bundle: %w", err)
		}

		// verify tsa signature
		tsaBytes, err := os.ReadFile("./devel/devkeys/tsa.crt")
		if err != nil {
			return fmt.Errorf("could not read devel/devkeys/tsa.crt: %w", err)
		}
		tsacerts, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(tsaBytes))
		if err != nil {
			return fmt.Errorf("could not load devel/devkeys/tsa.crt: %w", err)
		}
		tsacert := tsacerts[0]

		caBytes, err := os.ReadFile("./devel/devkeys/cacert.pem")
		if err != nil {
			return fmt.Errorf("could not read devel/devkeys/cacert.pem: %w", err)
		}
		caCerts, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(caBytes))
		if err != nil {
			return fmt.Errorf("could not load devel/devkeys/cacert.pem: %w", err)
		}

		for _, st := range signedTimestamps {
			_, err = verification.VerifyTimestampResponse(st, bytes.NewReader(sc.Signature()),
				verification.VerifyOpts{
					TSACertificate: tsacert,
					Roots:          caCerts,
				})
			if err != nil {
				return fmt.Errorf("could not verify timestamp: %w", err)
			}
		}
	}

	aki := fmt.Sprintf("%x", sha256.Sum256(signingCert.AuthorityKeyId))
	chain, ok := tr.Keys[aki]
	if !ok {
		return fmt.Errorf("trusted root not found for signing key with AKI %s", aki)
	}

	verifier, err := cosign.ValidateAndUnpackCertWithChain(signingCert, chain, &cosign.CheckOpts{IgnoreSCT: true})
	if err != nil {
		return fmt.Errorf("validating the certificate: %w", err)
	}

	dsseVerifier, err := dsse.NewEnvelopeVerifier(&sigdsee.VerifierAdapter{SignatureVerifier: verifier})
	if err != nil {
		return fmt.Errorf("creating DSSE verifier: %w", err)
	}

	_, err = dsseVerifier.Verify(ctx, attestation.DSSEEnvelopeFromBundle(bundle))
	return err
}

// old attestations have signatures base64 encoded twice
func fixSignatureInBundle(bundle *protobundle.Bundle) {
	sig := bundle.GetDsseEnvelope().GetSignatures()[0].GetSig()
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(sig)))
	i, err := base64.StdEncoding.Decode(dst, sig)
	if err == nil {
		// it was encoded twice.
		sig = dst[:i]
	}
	bundle.GetDsseEnvelope().GetSignatures()[0].Sig = sig
}
