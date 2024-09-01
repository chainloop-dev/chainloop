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
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type app struct {
	serverChan chan error
}

func newAuthLoginCmd() *cobra.Command {
	var forceHeadlessLogin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "authenticate the CLI with the Control Plane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return interactiveAuth(forceHeadlessLogin)
		},
	}

	cmd.Flags().BoolVar(&forceHeadlessLogin, "skip-browser", false, "perform a headless login process without opening a browser")
	return cmd
}

func interactiveAuth(forceHeadless bool) error {
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
	q.Set(oauth.QueryParamCallback, callbackURL.String())
	q.Set(oauth.QueryParamLongLived, "true")
	serverLoginURL.RawQuery = q.Encode()

	if forceHeadless {
		return headlessAuth(serverLoginURL)
	}

	err = openbrowser(serverLoginURL.String())
	if err != nil {
		logger.Debug().Err(err).Msg("falling back to manual login")
		return headlessAuth(serverLoginURL)
	}

	a.serverChan = make(chan error)

	// Run server in background
	http.HandleFunc(callbackURL.Path, a.handleCallback)
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
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
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
