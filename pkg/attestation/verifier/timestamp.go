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
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/timestamp-authority/pkg/verification"
)

func VerifyTimestamps(sb *bundle.Bundle, tr *TrustedRoot) error {
	sc, err := sb.SignatureContent()
	if err != nil {
		return fmt.Errorf("could not get signature material: %w", err)
	}

	signedTimestamps, err := sb.Timestamps()
	if err != nil {
		return fmt.Errorf("could not get timestamps from bundle: %w", err)
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

	var verifiedTimestamps int
	for _, st := range signedTimestamps {
		// let's try with all TSAs
		for _, tsa := range tr.TimestampAuthorities {
			var roots []*x509.Certificate
			var intermediates []*x509.Certificate
			if len(tsa) > 1 {
				roots = tsa[len(tsa)-1:]
				intermediates = tsa[1 : len(tsa)-1]
			}
			_, err = verification.VerifyTimestampResponse(st, bytes.NewReader(sigBytes),
				verification.VerifyOpts{
					TSACertificate: tsa[0],
					Intermediates:  intermediates,
					Roots:          roots,
				})
			if err != nil {
				continue
			}
			verifiedTimestamps++
		}
	}
	if verifiedTimestamps < len(signedTimestamps) {
		return fmt.Errorf("timestamps verification failed")
	}
	return nil
}
