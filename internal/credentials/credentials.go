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

package credentials

import (
	"context"
	"errors"
	"fmt"
)

type OCIKeypair struct {
	Repo, Username, Password string
}

type APICreds struct {
	Host, Key string
}

type ReaderWriter interface {
	Reader
	Writer
}

type Writer interface {
	SaveCredentials(ctx context.Context, org string, credentials any) (string, error)
	DeleteCredentials(ctx context.Context, credID string) error
}

type Reader interface {
	ReadCredentials(ctx context.Context, secretName string, credentials any) error
}

type Role int64

const (
	RoleReader Role = iota
	RoleWriter
)

var ErrNotFound = errors.New("credentials not found")
var ErrValidation = errors.New("credentials validation error")

// Validate that the OCIKeypair has all its properties set
func (o *OCIKeypair) Validate() error {
	if o.Repo == "" {
		return fmt.Errorf("%w: missing repo", ErrValidation)
	}
	if o.Username == "" {
		return fmt.Errorf("%w: missing username", ErrValidation)
	}
	if o.Password == "" {
		return fmt.Errorf("%w: missing password", ErrValidation)
	}

	return nil
}

// Validate that the APICreds has all its properties set
func (a *APICreds) Validate() error {
	if a.Host == "" {
		return fmt.Errorf("%w: missing host", ErrValidation)
	}
	if a.Key == "" {
		return fmt.Errorf("%w: missing key", ErrValidation)
	}
	return nil
}
