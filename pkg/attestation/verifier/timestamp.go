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
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/timestamp-authority/v2/pkg/verification"
)

var (
	// ErrTSAResponseInvalid indicates the RFC3161 timestamp response could not
	// be verified against the TSA certificate chain.
	ErrTSAResponseInvalid = errors.New("TSA response verification failed")

	// ErrTimestampOutsideTSAValidity indicates the timestamp's time falls
	// outside the TSA certificate's NotBefore/NotAfter window.
	ErrTimestampOutsideTSAValidity = errors.New("timestamp outside TSA certificate validity window")

	// ErrSigningCertNotValidAtTimestamp indicates the signing certificate
	// was not valid at the timestamp's time.
	ErrSigningCertNotValidAtTimestamp = errors.New("signing certificate not valid at timestamp time")

	// ErrNoTSARootsConfigured indicates the bundle contains signed timestamps
	// but no TSA trust roots are configured on the server.
	ErrNoTSARootsConfigured = errors.New("no TSA trust roots configured")

	// ErrTSASignerNotTrusted indicates the timestamp was signed by a certificate
	// that belongs to none of the configured timestamp authorities. Upstream TSAs
	// rotate their responder certificates without notice, so this points at a
	// pinned chain that has fallen behind, not at a faulty attestation.
	ErrTSASignerNotTrusted = errors.New("TSA response signer is not a configured timestamp authority")
)

// IsTrustConfigError reports whether err is a timestamp verification failure
// attributable to the TSA trust configuration rather than to the attestation
// itself. Such a failure must not reject an incoming attestation: the signature
// is verified independently, and verification is recomputed on every read, so
// the outcome self-heals once the configuration catches up with the upstream TSA.
func IsTrustConfigError(err error) bool {
	return errors.Is(err, ErrNoTSARootsConfigured) || errors.Is(err, ErrTSASignerNotTrusted)
}

func VerifyTimestamps(sb *bundle.Bundle, tr *TrustedRoot) error {
	signedTimestamps, err := sb.Timestamps()
	if err != nil {
		if errors.Is(err, bundle.ErrMissingVerificationMaterial) {
			return ErrMissingVerificationMaterial
		}
		return fmt.Errorf("could not get timestamps: %w", err)
	}
	if len(signedTimestamps) == 0 {
		return ErrMissingVerificationMaterial
	}

	if len(tr.TimestampAuthorities) == 0 {
		return ErrNoTSARootsConfigured
	}

	sc, err := sb.SignatureContent()
	if err != nil {
		return fmt.Errorf("could not get signature material: %w", err)
	}

	signature := sc.Signature()
	// See bug: https://github.com/chainloop-dev/chainloop/issues/1832
	// signature might be encoded twice. Let's try to fix it first.
	// TODO: remove this once the bug is fixed
	sigBytes := signature
	dst := make([]byte, base64.RawURLEncoding.DecodedLen(len(signature)))
	i, err := base64.StdEncoding.Decode(dst, signature)
	if err == nil {
		sigBytes = dst[:i]
	}

	vc, vcErr := sb.VerificationContent()
	if vcErr != nil && !errors.Is(vcErr, bundle.ErrMissingVerificationMaterial) {
		return fmt.Errorf("could not get verification material: %w", vcErr)
	}

	for _, st := range signedTimestamps {
		if err := verifyTimestamp(st, sigBytes, vc, tr); err != nil {
			return err
		}
	}
	return nil
}

// verifyTimestamp tries to verify a single signed timestamp against every
// configured TSA. Returns the error from the last attempted TSA on failure, or
// ErrTSASignerNotTrusted when the response was signed by none of them.
func verifyTimestamp(st []byte, sigBytes []byte, vc verify.VerificationContent, tr *TrustedRoot) error {
	// Chainloop's timestamp requests do not ask for certificates, so the response
	// carries none and the pinned leaf is injected as the only candidate signer.
	// Knowing up front which authority actually signed keeps a rotated upstream
	// responder distinguishable from a response we should reject.
	signers := tsrSigners(st)

	var lastErr error
	var skipped []string
	for name, tsa := range tr.TimestampAuthorities {
		tsaCert := tsa[0]
		if !signedByCert(signers, tsaCert) {
			skipped = append(skipped, fmt.Sprintf("%q (expected leaf %q, serial %s)",
				name, tsaCert.Subject.CommonName, tsaCert.SerialNumber))
			continue
		}

		var roots []*x509.Certificate
		var intermediates []*x509.Certificate
		if len(tsa) > 1 {
			roots = tsa[len(tsa)-1:]
			intermediates = tsa[1 : len(tsa)-1]
		}

		ts, err := verification.VerifyTimestampResponse(st, bytes.NewReader(sigBytes),
			verification.VerifyOpts{
				TSACertificate: tsaCert,
				Intermediates:  intermediates,
				Roots:          roots,
			})
		if err != nil {
			lastErr = fmt.Errorf("%w: %w", ErrTSAResponseInvalid, err)
			continue
		}

		if ts.Time.After(tsaCert.NotAfter) || ts.Time.Before(tsaCert.NotBefore) {
			lastErr = fmt.Errorf("%w: timestamp=%s, cert validity=[%s, %s]",
				ErrTimestampOutsideTSAValidity, ts.Time, tsaCert.NotBefore, tsaCert.NotAfter)
			continue
		}

		if vc != nil && vc.Certificate() != nil && !vc.ValidAtTime(ts.Time, nil) {
			lastErr = fmt.Errorf("%w: timestamp=%s", ErrSigningCertNotValidAtTimestamp, ts.Time)
			continue
		}

		return nil
	}

	// No authority signed this response: our pinned chains are behind the
	// upstream TSA rather than the response being at fault.
	if lastErr == nil && len(skipped) > 0 {
		return fmt.Errorf("%w: tried %s", ErrTSASignerNotTrusted, strings.Join(skipped, ", "))
	}

	return lastErr
}

// tsrSignerID identifies the certificate that signed an RFC3161 response, as
// carried in the PKCS#7 SignerInfo.
type tsrSignerID struct {
	rawIssuer []byte
	serial    *big.Int
}

// tsrSigners returns the identities of the certificates that signed the RFC3161
// response. A nil result means the response could not be parsed; callers must
// then treat every candidate as a possible signer so the underlying verifier
// produces the authoritative error.
func tsrSigners(st []byte) []tsrSignerID {
	ts, err := timestamp.ParseResponse(st)
	if err != nil {
		return nil
	}

	p7, err := pkcs7.Parse(ts.RawToken)
	if err != nil || len(p7.Signers) == 0 {
		return nil
	}

	signers := make([]tsrSignerID, 0, len(p7.Signers))
	for _, signer := range p7.Signers {
		signers = append(signers, tsrSignerID{
			rawIssuer: signer.IssuerAndSerialNumber.IssuerName.FullBytes,
			serial:    signer.IssuerAndSerialNumber.SerialNumber,
		})
	}

	return signers
}

// signedByCert reports whether cert is one of the given signers. An unknown
// signer set (a response we could not parse) matches every certificate.
func signedByCert(signers []tsrSignerID, cert *x509.Certificate) bool {
	if signers == nil {
		return true
	}

	for _, signer := range signers {
		if signer.serial != nil && cert.SerialNumber.Cmp(signer.serial) == 0 && bytes.Equal(cert.RawIssuer, signer.rawIssuer) {
			return true
		}
	}

	return false
}
