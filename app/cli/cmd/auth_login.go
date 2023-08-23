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
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

type app struct {
	serverChan chan error
}

func newAuthLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "authenticate the CLI with the Control Plane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return interactiveAuth()
		},
	}

	return cmd
}

func interactiveAuth() error {
	var a app

	listener, callbackURL, err := localListenerAndCallbackURL()
	if err != nil {
		return err
	}
	// Get the auth login from the control plane directly
	authURL, err := retrieveLoginURL()
	if err != nil {
		return err
	}

	serverLoginURL, err := url.Parse(authURL)
	if err != nil {
		return err
	}
	// Append local callback URL
	q := serverLoginURL.Query()
	q.Set("callback", callbackURL.String())
	q.Set("long-lived", "true")
	serverLoginURL.RawQuery = q.Encode()

	err = openbrowser(serverLoginURL.String())
	if err != nil {
		logger.Debug().Err(err).Msg("falling back to manual login")
		return headlessAuth(serverLoginURL)
	}

	a.serverChan = make(chan error)

	// Run server in background
	http.HandleFunc(callbackURL.Path, a.handleCallback)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	go func() {
		logger.Info().Msg("waiting for the authentication to be completed, please check your browser")

		server := &http.Server{ReadHeaderTimeout: time.Second}

		err := server.Serve(listener)
		if err != nil {
			log.Fatal(err)
		}
	}()

	err = <-a.serverChan
	if err != nil {
		return err
	}

	return nil
}

func headlessAuth(loginURL *url.URL) error {
	// Remove cli-callback query parameter to indicate the server to show it inline
	q := loginURL.Query()
	q.Del("callback")
	loginURL.RawQuery = q.Encode()
	fmt.Printf("To authenticate, click on the following link and paste the result back here\n\n  %s\n\n", loginURL.String())

	fmt.Print("Enter Token: ")
	token, err := term.ReadPassword(syscall.Stdin)
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

// Save token to config file
func saveAuthToken(token string) error {
	viper.Set(confOptions.authToken.viperKey, token)
	return viper.WriteConfig()
}

func (a *app) handleCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("t")

	defer func() {
		a.serverChan <- nil
	}()

	if token == "" {
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	if err := saveAuthToken(token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Info().Msg("login successful!")
	fmt.Fprintln(w, "login successful, you can now close this window and go back to the terminal")
}

func openbrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// Retrieve loginURL from the control plane
func retrieveLoginURL() (string, error) {
	client := pb.NewStatusServiceClient(actionOpts.CPConnection)
	resp, err := client.Infoz(context.Background(), &pb.InfozRequest{})
	if err != nil {
		return "", err
	}
	return resp.LoginUrl, nil
}

// Create a local HTTP listener with a random available port
func localListenerAndCallbackURL() (net.Listener, *url.URL, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil, err
	}

	// URL to where the server will post back the auth token
	callbackURL := &url.URL{Scheme: "http", Host: listener.Addr().String(), Path: "/auth/callback"}
	return listener, callbackURL, nil
}
