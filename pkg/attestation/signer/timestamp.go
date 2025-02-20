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

package signer

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/sign"
)

type TimestampSigner struct {
	tsaURL string
}

func NewTimestampSigner(tsaURL string) *TimestampSigner {
	return &TimestampSigner{tsaURL: tsaURL}
}

func (ts *TimestampSigner) SignMessage(ctx context.Context, payload []byte) ([]byte, error) {
	// TSA signature
	tsa := sign.NewTimestampAuthority(&sign.TimestampAuthorityOptions{
		URL: ts.tsaURL,
	})

	// See bug: https://github.com/chainloop-dev/chainloop/issues/1832
	// signature might be encoded twice. Let's try to fix it first.
	// TODO: remove this once the bug is fixed
	toSign := payload
	dst := make([]byte, base64.RawURLEncoding.DecodedLen(len(payload)))
	i, err := base64.StdEncoding.Decode(dst, payload)
	if err == nil {
		// get the decoded
		toSign = dst[:i]
	}

	tsaSig, err := tsa.GetTimestamp(ctx, toSign)
	if err != nil {
		return nil, fmt.Errorf("getting timestamp signature: %w", err)
	}
	return tsaSig, nil
}
