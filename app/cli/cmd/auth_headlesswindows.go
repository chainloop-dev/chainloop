//go:build windows
// +build windows

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

package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"syscall"

	jwt "github.com/golang-jwt/jwt/v4"
	"golang.org/x/term"
)

func headlessAuth(loginURL *url.URL) error {
	// Remove cli-callback query parameter to indicate the server to show it inline
	q := loginURL.Query()
	q.Set("callback", "")
	loginURL.RawQuery = q.Encode()
	fmt.Printf("To authenticate, click on the following link and paste the result back here\n\n  %s\n\n", loginURL.String())

	fmt.Print("Enter Token: ")
	// This is windows specific code
	fd := int(syscall.Handle(os.Stdin.Fd()))

	token, err := term.ReadPassword(fd)
	if err != nil {
		return fmt.Errorf("retrieving password from stdin: %w", err)
	}

	// We just want to check that it is a token, the actual verification will happen when it is sent to the server
	// To be clear, this is just a best effort sanity check
	if _, _, err := new(jwt.Parser).ParseUnverified(string(token), &jwt.MapClaims{}); err != nil {
		return errors.New("invalid token")
	}

	if err := saveAuthToken(string(token)); err != nil {
		return fmt.Errorf("storing token in config file: %w", err)
	}

	fmt.Println("")
	logger.Info().Msg("login successful!")

	return nil
}
