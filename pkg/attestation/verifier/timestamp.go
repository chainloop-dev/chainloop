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
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/digitorus/timestamp"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/timestamp-authority/pkg/verification"
)

func VerifyTimestamps(sb *bundle.Bundle, tr *TrustedRoot) error {
	signedTimestamps, err := sb.Timestamps()
	if err != nil {
		return err
	}
	if len(signedTimestamps) == 0 {
		// nothing to do
		return nil
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
		// get the decoded one
		sigBytes = dst[:i]
	}

	var verifiedTimestamps []*timestamp.Timestamp
	for _, st := range signedTimestamps {
		// let's try with all TSAs
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
				continue
			}
			// verify timestamp time
			if ts.Time.After(tsaCert.NotAfter) || ts.Time.Before(tsaCert.NotBefore) {
				continue
			}

			vc, err := sb.VerificationContent()
			if err != nil && !errors.Is(err, bundle.ErrMissingVerificationMaterial) {
				return fmt.Errorf("could not get verification material: %w", err)
			}
			// verify signing certificate issuing time
			if vc != nil && vc.GetCertificate() != nil && !vc.ValidAtTime(ts.Time, nil) {
				continue
			}
			verifiedTimestamps = append(verifiedTimestamps, ts)
		}
	}
	if len(verifiedTimestamps) < len(signedTimestamps) {
		return fmt.Errorf("some timestamps verification failed")
	}
	return nil
}
