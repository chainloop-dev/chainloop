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
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	bundle2 "github.com/sigstore/sigstore-go/pkg/bundle"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
)

type TrustedRoot struct {
	// map key identifiers to a chain of certificates
	Keys map[string][]*x509.Certificate
}

func VerifyBundle(ctx context.Context, bundleBytes []byte, tr *TrustedRoot) error {
	var bundle bundle2.Bundle
	bundle.Bundle = new(protobundle.Bundle)
	// unmarshal and validate
	if err := bundle.UnmarshalJSON(bundleBytes); err != nil {
		return err
	}
	pb := bundle.Bundle
	if pb.GetVerificationMaterial() == nil || pb.GetVerificationMaterial().GetCertificate() == nil {
		// nothing to verify
		return nil
	}

	rawCert := pb.GetVerificationMaterial().GetCertificate().GetRawBytes()
	signingCert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return err
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

	_, err = dsseVerifier.Verify(ctx, attestation.DSSEEnvelopeFromBundle(pb))
	return err
}
