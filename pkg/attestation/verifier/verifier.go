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
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v3/pkg/cosign"
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
var ErrInvalidBundle = errors.New("invalid bundle")

// ErrUnsupportedVerificationMaterial indicates the bundle carries verification
// material we cannot verify a signature against (e.g. a bare public key with no
// trusted key set). It is treated as a verification failure, never ignored.
var ErrUnsupportedVerificationMaterial = errors.New("unsupported verification material")

func VerifyBundle(ctx context.Context, bundleBytes []byte, tr *TrustedRoot) error {
	if bundleBytes == nil {
		return ErrMissingVerificationMaterial
	}

	bundle := new(protobundle.Bundle)
	// unmarshal and validate
	if err := protojson.Unmarshal(bundleBytes, bundle); err != nil {
		return fmt.Errorf("%w: %w", err, ErrInvalidBundle)
	}

	// fix for old attestations
	attestation.FixSignatureInBundle(bundle)

	sb := &sigstorebundle.Bundle{Bundle: bundle}
	vc, err := sb.VerificationContent()
	if err != nil {
		if !errors.Is(err, sigstorebundle.ErrMissingVerificationMaterial) {
			return fmt.Errorf("could not get verification material: %w", err)
		}
	}

	// Signature verification is MANDATORY
	switch {
	case vc != nil && vc.Certificate() != nil:
		if err := verifyCertSignature(ctx, bundle, vc.Certificate(), tr); err != nil {
			return err
		}
	case bundle.GetVerificationMaterial().GetPublicKey() != nil:
		// Public-key bundles are not supported at this time
		return fmt.Errorf("%w: public key verification material", ErrUnsupportedVerificationMaterial)
	default:
		// No certificate and no public key: nothing to verify the signature against.
		return ErrMissingVerificationMaterial
	}

	// The signature has been verified against a trusted certificate. The timestamp
	// (if present) only validates the signing window; it can never be the sole
	// verification material.
	if err := VerifyTimestamps(sb, tr); err != nil && !errors.Is(err, ErrMissingVerificationMaterial) {
		return fmt.Errorf("could not verify timestamps: %w", err)
	}

	return nil
}

// verifyCertSignature validates the signing certificate against the trusted root
// chain and verifies the DSSE envelope signature with the certificate's key.
func verifyCertSignature(ctx context.Context, bundle *protobundle.Bundle, signingCert *x509.Certificate, tr *TrustedRoot) error {
	akiSum := sha256.Sum256(signingCert.AuthorityKeyId)
	aki := hex.EncodeToString(akiSum[:])
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

	if _, err := dsseVerifier.Verify(ctx, attestation.DSSEEnvelopeFromBundle(bundle)); err != nil {
		return fmt.Errorf("validating the DSSE envelope: %w", err)
	}

	return nil
}
