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

package cmd

import (
	"errors"
)

// GracefulError represents an error that has been marked as gracefully handled
// In some parts of our code, we want to raise errors but we don't want the CLI to fail
// because of the flakiness or active development of the tool
type GracefulError struct {
	err error
}

func (e GracefulError) Error() string {
	return e.err.Error()
}

func newGracefulError(err error) error {
	return GracefulError{err}
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized, execute the init command first")
var ErrAttestationAlreadyExist = errors.New("attestation already initialized, to override it use the --replace flag`")
var ErrAttestationTokenRequired = errors.New("token required, please provide it via the pre-defined env variable or command flag")
var ErrKeylessNotSupported = errors.New("keyless signing not supported, please provide a private key reference with --key instead")
