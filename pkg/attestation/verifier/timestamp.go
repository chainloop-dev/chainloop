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
)

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
// configured TSA. Returns the error from the last attempted TSA on failure.
func verifyTimestamp(st []byte, sigBytes []byte, vc verify.VerificationContent, tr *TrustedRoot) error {
	var lastErr error
	for _, tsa := range tr.TimestampAuthorities {
		tsaCert := tsa[0]
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
	return lastErr
}
