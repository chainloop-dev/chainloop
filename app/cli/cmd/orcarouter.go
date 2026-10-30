//
// Copyright 2026 The Chainloop Authors.
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
	"io"
	"os"
	"strings"

	"github.com/chainloop-dev/chainloop/internal/orcarouter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Configuration keys of the OrcaRouter credential. It lives next to the
// existing CLI settings, in the same file the other credentials use, so no
// second secret store is introduced.
const (
	// viperKeyOrcaKey is the stored API key.
	viperKeyOrcaKey = "orcarouter.api-key"
	// viperKeyOrcaSource records which adapter produced the stored key.
	viperKeyOrcaSource = "orcarouter.credential-source"
)

// newOrcaRouterCmd creates the `orcarouter` command group. Both credential
// paths are discoverable subcommands: `login` pastes an existing key, `connect`
// runs the OAuth 2.0 + PKCE flow. They end at the same stored credential.
func newOrcaRouterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   orcarouter.ID,
		Short: "Configure " + orcarouter.Name,
		Long: orcarouter.Name + ` is an OpenAI-compatible inference gateway for security teams.

Cyber models for your red team, gateway controls for your blue team: it admits
authorized, scoped security inference that general gateways often refuse or
silently downgrade. See ` + orcarouter.DocsURL + ` and the cyber model catalog at
` + orcarouter.ModelsURL + `.

Two credential paths end at the same durable API key, and either one works on
its own:

  chainloop orcarouter login       paste an existing sk-orca-... key
  chainloop orcarouter connect     authorize in a browser with OAuth 2.0 + PKCE

Authentication uses ` + orcarouter.AuthBaseURL + `; inference and model discovery use
` + orcarouter.BaseURL + `.`,
		Annotations: map[string]string{skipActionOptsInit: trueString},
	}

	cmd.AddCommand(
		newOrcaRouterLoginCmd(),
		newOrcaRouterConnectCmd(),
		newOrcaRouterStatusCmd(),
		newOrcaRouterLogoutCmd(),
		newOrcaRouterModelsCmd(),
		newOrcaRouterChatCmd(),
	)

	return cmd
}

// orcaRouterManager builds the credential manager over the CLI configuration
// file.
func orcaRouterManager() (*orcarouter.Manager, error) {
	return orcarouter.NewManager(viperStore{})
}

// viperStore persists the credential in the CLI configuration file, the store
// the other credentials already use.
type viperStore struct{}

// Load implements orcarouter.Store.
func (viperStore) Load() (string, orcarouter.Source, error) {
	key := strings.TrimSpace(viper.GetString(viperKeyOrcaKey))
	source := orcarouter.Source(viper.GetString(viperKeyOrcaSource))
	return key, source, nil
}

// Save implements orcarouter.Store.
func (viperStore) Save(key string, source orcarouter.Source) error {
	viper.Set(viperKeyOrcaKey, key)
	viper.Set(viperKeyOrcaSource, string(source))
	return viper.WriteConfig()
}

// Clear implements orcarouter.Store. The key is overwritten with an empty
// value and the source is dropped, so no credential remains in the file.
func (viperStore) Clear() error {
	viper.Set(viperKeyOrcaKey, "")
	viper.Set(viperKeyOrcaSource, "")
	return viper.WriteConfig()
}

// newOrcaRouterLoginCmd pastes an existing API key.
func newOrcaRouterLoginCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an existing " + orcarouter.Name + " API key",
		Long: `Store an existing ` + orcarouter.Name + ` API key.

Create or copy a key at ` + orcarouter.DocsURL + `. The key is checked for its
documented shape only: the first request is what proves it works. It is written
to the CLI configuration file and is never printed.
`,
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, _ []string) error {
			secret := key
			if secret == "" {
				secret = strings.TrimSpace(os.Getenv(orcarouter.EnvKey))
			}
			if secret == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Paste the %s API key: ", orcarouter.Name)
				entered, err := readPasswordFromTerminal()
				if err != nil {
					return fmt.Errorf("read the API key: %w", err)
				}
				secret = strings.TrimSpace(string(entered))
			}

			manager, err := orcaRouterManager()
			if err != nil {
				return err
			}
			cred, err := manager.SetAPIKey(secret)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored %s credential %s (source: %s)\n", orcarouter.Name, cred.Redacted().Key, cred.Source)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "API key to store; reads $"+orcarouter.EnvKey+" or prompts when empty")
	return cmd
}

// newOrcaRouterConnectCmd runs the OAuth 2.0 + PKCE login.
func newOrcaRouterConnectCmd() *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Authorize with an " + orcarouter.Name + " account (OAuth 2.0 + PKCE)",
		Long: `Authorize this machine with an ` + orcarouter.Name + ` account.

The login opens a browser, receives the code on a loopback redirect, and
exchanges it for a normal API key that belongs to your account: it is billed to
you and revocable at ` + orcarouter.ConsoleURL + `. No client secret is involved,
because PKCE binds the code to this process.

The key is durable and is reused until it is revoked. Re-authorizing on every
launch would exhaust the per-user issuance limit.

When no browser can be opened, the authorization URL is printed so it can be
visited elsewhere; the loopback callback still receives the result.
`,
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := orcaRouterManager()
			if err != nil {
				return err
			}

			opts := orcarouter.ConnectOptions{AppName: "Chainloop"}
			if noBrowser {
				opts.OpenBrowser = func(u string) error {
					fmt.Fprintf(cmd.OutOrStdout(), "Open this URL to authorize:\n\n  %s\n\n", u)
					return nil
				}
			}
			cred, err := orcarouter.Connect(cmd.Context(), opts)
			if err != nil {
				return describeConnectError(err)
			}
			stored, err := manager.SetPKCE(cred)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Connected. Stored %s credential %s (scope: %s)\n", orcarouter.Name, stored.Redacted().Key, stored.Scope)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the authorization URL instead of opening a browser")
	return cmd
}

// describeConnectError turns a login failure into an actionable message without
// echoing any response body.
func describeConnectError(err error) error {
	switch {
	case errors.Is(err, orcarouter.ErrAccessDenied):
		return errors.New("authorization was declined; nothing was stored")
	case errors.Is(err, orcarouter.ErrStateMismatch):
		return errors.New("the authorization response did not belong to this attempt; run the command again")
	case errors.Is(err, orcarouter.ErrExchangeRejected):
		return errors.New("the authorization code was rejected or already used; run the command again")
	case errors.Is(err, orcarouter.ErrScopeDowngrade):
		return fmt.Errorf("%w; the account cannot grant the requested scope", err)
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("timed out waiting for authorization; run the command again")
	default:
		return err
	}
}

// newOrcaRouterStatusCmd shows the credential without revealing it.
func newOrcaRouterStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show the configured " + orcarouter.Name + " credential",
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := orcaRouterManager()
			if err != nil {
				return err
			}
			cred, ok := manager.Current()
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "No %s credential. Run `chainloop orcarouter login` or `chainloop orcarouter connect`.\n", orcarouter.Name)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Provider:   %s\nBase URL:   %s\nCredential: %s\nSource:     %s\nGeneration: %d\nNeeds reauth: %t\n",
				orcarouter.Name, orcarouter.APIOrigin(), cred.Redacted().Key, cred.Source, cred.Generation, manager.NeedsReauth())
			return nil
		},
	}
}

// newOrcaRouterLogoutCmd clears the stored credential.
func newOrcaRouterLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "logout",
		Short:       "Remove the stored " + orcarouter.Name + " credential",
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := orcaRouterManager()
			if err != nil {
				return err
			}
			if err := manager.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Removed the stored credential. Revoke the key itself at "+orcarouter.ConsoleURL+".")
			return nil
		},
	}
}

// newOrcaRouterModelsCmd lists the live catalog for one capability, so the
// filtering rules are visible from the terminal.
func newOrcaRouterModelsCmd() *cobra.Command {
	var (
		capability string
		imageInput bool
	)
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the " + orcarouter.Name + " models available to this credential",
		Long: `List the models that ` + orcarouter.BaseURL + `/models returns for this credential,
filtered for one entry point.

The live catalog is authoritative. When it cannot be reached, a small verified
fallback list is shown and marked degraded, so a fresh installation is not left
with an empty selector.
`,
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := orcaRouterManager()
			if err != nil {
				return err
			}
			cred, ok := manager.Current()
			if !ok {
				return fmt.Errorf("%w; run `chainloop orcarouter login` or `chainloop orcarouter connect`", orcarouter.ErrNoCredential)
			}

			cat := orcarouter.DiscoverCatalog(cmd.Context(), nil, orcarouter.APIOrigin(), cred)
			if cat.Degraded {
				fmt.Fprintf(cmd.OutOrStderr(), "Live discovery failed (%v); showing the verified fallback catalog.\n", cat.Err)
			}

			var required []string
			if imageInput {
				required = append(required, "image")
			}
			models := orcarouter.ForCapability(cat.Models, orcarouter.Capability(capability), required)

			if len(models) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No models for capability %q.\n", capability)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s, %d models)\n", orcarouter.Name, cat.Source, len(models))
			for _, model := range models {
				fmt.Fprintln(cmd.OutOrStdout(), model.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&capability, "capability", string(orcarouter.CapabilityChat), "capability to list: chat, embedding, image, video, rerank")
	cmd.Flags().BoolVar(&imageInput, "image-input", false, "only models that declare image input")
	return cmd
}

// newOrcaRouterChatCmd sends one prompt through the provider. It is the
// smallest end-to-end path through the inference client, for verifying a
// credential and the wiring without an agent harness.
func newOrcaRouterChatCmd() *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "chat <prompt>",
		Short: "Send one prompt through " + orcarouter.Name,
		Long: `Send one prompt to ` + orcarouter.BaseURL + ` with the stored credential.

The model is an ID from ` + orcarouter.ModelsURL + `; it defaults to
` + orcarouter.DefaultModel + `, which routes automatically. Inference and model
discovery use the inference origin, never the authentication origin.
`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{skipActionOptsInit: trueString},
		RunE: func(cmd *cobra.Command, args []string) error {
			return orcaRouterChat(cmd.Context(), cmd.OutOrStdout(), model, args[0])
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "model ID (default "+orcarouter.DefaultModel+")")
	return cmd
}

// orcaRouterChat sends one chat request through the provider. It exists so the
// wiring of the inference path can be exercised without another harness.
func orcaRouterChat(ctx context.Context, out io.Writer, model, prompt string) error {
	manager, err := orcaRouterManager()
	if err != nil {
		return err
	}
	cred, ok := manager.Current()
	if !ok {
		return fmt.Errorf("%w; run `chainloop orcarouter login` or `chainloop orcarouter connect`", orcarouter.ErrNoCredential)
	}
	if model == "" {
		model = orcarouter.DefaultModel
	}

	generation := manager.Generation()
	client := &orcarouter.Client{}
	answer, err := client.Chat(ctx, cred, orcarouter.ChatRequest{
		Model:    model,
		Messages: []orcarouter.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		// A rejected durable key needs a new login; it is not refreshable.
		if errors.Is(err, orcarouter.ErrNeedsReauth) {
			manager.Rejected(generation)
			return fmt.Errorf("%w: run `chainloop orcarouter connect` or `chainloop orcarouter login`", err)
		}
		return err
	}
	fmt.Fprintln(out, answer)
	return nil
}
