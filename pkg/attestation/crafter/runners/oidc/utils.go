//
// Copyright 2023 The Chainloop Authors.
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
	"sort"
	"time"
)

// OIDCToken represents the contents of a GitHub OIDC JWT token.
type OIDCToken struct {
	// Expiry is the expiration date of the token.
	Expiry time.Time

	// Issuer is the token issuer.
	Issuer string

	// JobWorkflowRef is a reference to the current job workflow.
	JobWorkflowRef string `json:"job_workflow_ref"`

	// RunnerEnvironment is the environment the runner is running in.
	RunnerEnvironment string `json:"runner_environment"`

	// RawToken is the unparsed oidc token.
	RawToken string

	// Audience is the audience for which the token was granted.
	Audience []string
}

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

// OIDCClient is the interface for an OIDC client.
type OIDCClient interface {
	Token(ctx context.Context) (*OIDCToken, error)
}

// NoOPClient is a empty implementation of OIDCClient that returns an empty token.
type NoOPClient struct{}

func (r *NoOPClient) Token(ctx context.Context) (*OIDCToken, error) {
	return &OIDCToken{
		Expiry:            time.Now(),
		RunnerEnvironment: "",
		Issuer:            "",
		JobWorkflowRef:    "",
	}, nil
}

func NewNoOPClient() OIDCClient {
	return &NoOPClient{}
}

func compareStringSlice(s1, s2 []string) bool {
	// Verify the audience received is the one we requested.
	if len(s1) != len(s2) {
		return false
	}

	c1 := append([]string{}, s1...)
	sort.Strings(c1)

	c2 := append([]string{}, s2...)
	sort.Strings(c2)

	for i := range c1 {
		if c1[i] != c2[i] {
			return false
		}
	}

	return true
}
