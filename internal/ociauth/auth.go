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

package ociauth

import (
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

var UserPasswordKeyChain authn.Keychain = &Credentials{}

type Credentials struct {
	username, password, server string
}

func NewCredentials(repoURI, username, password string) (authn.Keychain, error) {
	repo, err := name.NewRepository(repoURI)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URI: %w", err)
	}

	// NOTE: NewRepository parses incorrectly URIs with schemas
	c := &Credentials{username, password, repo.RegistryStr()}
	return validateOCICredentials(c)
}

// Resolve implements an authn.KeyChain
//
// See https://pkg.go.dev/github.com/google/go-containerregistry/pkg/authn#Keychain
//
// Returns a custom credentials authn.Authenticator if the given resource
// RegistryStr() matches the Repository, otherwise it returns annonymous access
func (repo *Credentials) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if repo.server == resource.RegistryStr() {
		return repo, nil
	}

	// if no credentials are provided we return annon authentication
	return authn.Anonymous, nil
}

// Authorization implements an authn.Authenticator
//
// See https://pkg.go.dev/github.com/google/go-containerregistry/pkg/authn#Authenticator
//
// Returns an authn.AuthConfig with a user / password pair to be used for authentication
func (repo *Credentials) Authorization() (*authn.AuthConfig, error) {
	return &authn.AuthConfig{Username: repo.username, Password: repo.password}, nil
}

// validate if the provided OCI credentials are valid
// They include a username, password and a valid (RFC 3986 URI authority) serverName
func validateOCICredentials(c *Credentials) (authn.Keychain, error) {
	if c.username == "" || c.password == "" || c.server == "" {
		return nil, errors.New("OCI credentials require an username, password and a server name")
	}

	// name.NewRepository parses incorrectly URIs with schemas
	// name.NewRegistry does it correctly but that will break the fact that we authenticate using full paths
	if c.server == "http:" || c.server == "https:" {
		return nil, errors.New("credentials server name must not contain a scheme (\"http://\")")
	}

	return c, nil
}
