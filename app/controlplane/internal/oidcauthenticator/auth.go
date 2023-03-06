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

package authenticator

import (
	"context"
	"errors"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDC is used to authenticate our users.
type OIDC struct {
	*oidc.Provider
	oauth2.Config
}

// NewOIDC instantiates an OIDC authenticator.
// During initialization the endpoints are retrieved from the discovery URL
func NewOIDC(discoveryDomain, clientID, clientSecret, redirectURL string) (*OIDC, error) {
	provider, err := oidc.NewProvider(context.Background(), discoveryDomain)
	if err != nil {
		return nil, err
	}

	conf := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &OIDC{
		Provider: provider,
		Config:   conf,
	}, nil
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
// Including checking for it's sigature, expiration, audience and so on.
func (a *OIDC) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in oauth2 token")
	}

	// default settings
	oidcConfig := &oidc.Config{ClientID: a.ClientID}

	return a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
}
