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

package oidc

import (
	"context"
	"errors"
)

const SelfHostedRunner = "self-hosted"

var (
	// errURLError indicates the OIDC server URL is invalid.
	errURLError = errors.New("url")

	// errRequestError indicates an error requesting the token from the issuer.
	errRequestError = errors.New("http request")

	// errToken indicates an error in the format of the token.
	errToken = errors.New("token")

	// errClaims indicates an error in the claims of the token.
	errClaims = errors.New("claims")

	// errVerify indicates an error in the token verification process.
	errVerify = errors.New("verify")
)

// Client is the interface for an OIDC client.
type Client interface {
	Token(ctx context.Context) (any, error)
}
