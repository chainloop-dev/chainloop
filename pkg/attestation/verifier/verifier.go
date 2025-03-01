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
	"context"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
	"google.golang.org/protobuf/encoding/protojson"
)

type TrustedRoot struct {
	// map key identifiers to a chain of certificates
	Keys                 map[string][]*x509.Certificate
	TimestampAuthorities map[string][]*x509.Certificate
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

	hasVerificationMaterial := false
	sb := &sigstorebundle.Bundle{Bundle: bundle}
	vc, err := sb.VerificationContent()
	if err != nil {
		if !errors.Is(err, sigstorebundle.ErrMissingVerificationMaterial) {
			return fmt.Errorf("could not get verification material: %w", err)
		}
	}

	if vc != nil && vc.GetCertificate() != nil {
		hasVerificationMaterial = true
		signingCert := vc.GetCertificate()

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
		if err != nil {
			return fmt.Errorf("validating the DSSE envelope: %w", err)
		}
	}

	// Even with no cert (using a local key), we can still validate the timestamp
	if err = VerifyTimestamps(sb, tr); err != nil {
		if !errors.Is(err, ErrMissingVerificationMaterial) {
			return fmt.Errorf("could not verify timestamps: %w", err)
		}
	} else {
		hasVerificationMaterial = true
	}

	if !hasVerificationMaterial {
		return ErrMissingVerificationMaterial
	}

	return nil
}
