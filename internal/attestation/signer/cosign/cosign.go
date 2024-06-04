//
// Copyright 2024 The Chainloop Authors.
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

package cosign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/sigstore/cosign/v2/pkg/signature"
	sigstoresigner "github.com/sigstore/sigstore/pkg/signature"
	"golang.org/x/term"
)

// Signer is a signer leveraging cosign
type Signer struct {
	sigstoresigner.Signer
	keyPath string
	logger  zerolog.Logger

	mu sync.Mutex
}

func NewSigner(keyPath string, logger zerolog.Logger) *Signer {
	return &Signer{
		keyPath: keyPath, logger: logger,
	}
}

func (cs *Signer) SignMessage(message io.Reader, opts ...sigstoresigner.SignOption) ([]byte, error) {
	if err := cs.ensureInitiated(); err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	return cs.Signer.SignMessage(message, opts...)
}

// ensureInitiated makes sure that a proper cosign signer has been created from the key reference
func (cs *Signer) ensureInitiated() error {
	var err error

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.Signer == nil {
		cs.logger.Debug().Str("path", cs.keyPath).Msg("loading key")
		cs.Signer, err = signature.SignerFromKeyRef(context.Background(), cs.keyPath, getPass)
		if err != nil {
			return fmt.Errorf("creating signer from keyref: %w", err)
		}
	}
	return nil
}

func getPass(confirm bool) ([]byte, error) {
	read := readPasswordFn(confirm)
	return read()
}

// based on cosign code
// https://pkg.go.dev/github.com/sigstore/cosign/cmd/cosign/cli/generate
func readPasswordFn(confirm bool) func() ([]byte, error) {
	pw, ok := os.LookupEnv("CHAINLOOP_SIGNING_PASSWORD")
	switch {
	case ok:
		return func() ([]byte, error) {
			return []byte(pw), nil
		}
	case isTerminal():
		return func() ([]byte, error) {
			return getPassFromTerm(confirm)
		}
	// Handle piped in passwords.
	default:
		return func() ([]byte, error) {
			return io.ReadAll(os.Stdin)
		}
	}
}

func isTerminal() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func getPassFromTerm(confirm bool) ([]byte, error) {
	fmt.Fprint(os.Stderr, "Enter password for private key: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr)
	if !confirm {
		return pw1, nil
	}
	fmt.Fprint(os.Stderr, "Enter password for private key again: ")
	// Unnecessary convert of syscall.Stdin on *nix, but Windows is a uintptr
	// nolint:unconvert
	confirmpw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	if string(pw1) != string(confirmpw) {
		return nil, errors.New("passwords do not match")
	}
	return pw1, nil
}
