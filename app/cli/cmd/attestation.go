//
// Copyright 2024-2026 The Chainloop Authors.
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
	"fmt"
	"os"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
	internaltoken "github.com/chainloop-dev/chainloop/app/cli/internal/token"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	attestationOrganizationNameAnnotation = "chainloop.attestation.organization"
	attestationOrganizationFileAnnotation = "chainloop.attestation.organization-file"
)

var repositoryOrganizationCommands = map[string]bool{
	"init": true, "add": true, "push": true, "status": true, "reset": true,
}

var (
	useAttestationRemoteState bool
	attestationLocalStatePath string
	GracefulExit              bool
	// attestationID is the unique identifier of the in-progress attestation
	// this is required when use-attestation-remote-state is enabled
	attestationID string
)

func newAttestationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attestation",
		Aliases: []string{"att"},
		Short:   "Craft Software Supply Chain Attestations",
		Example: "Refer to https://docs.chainloop.dev/getting-started/attestation-crafting",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveAttestationOrganization(cmd); err != nil {
				return err
			}

			// run the initialization of the root command plus the new logic
			// specific to this attestation command
			rootCmd := cmd.Parent().Parent()
			if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
				return err
			}

			// If the subcommand has the attestation-id flag,
			// we need to make sure that it's set if the remote-state flag is enabled
			if useAttestationRemoteState && cmd.Flags().Lookup("attestation-id") != nil {
				return cmd.MarkFlagRequired("attestation-id")
			}

			return nil
		},
	}

	cmd.PersistentFlags().Bool("remote-state", false, "Store the attestation state remotely (preview feature)")
	// We do not need this flag in all the attestation subcommands just in init, but we don't want to remove it to not to break current integrations
	cobra.CheckErr(cmd.PersistentFlags().MarkHidden("remote-state"))

	cmd.PersistentFlags().BoolVar(&GracefulExit, "graceful-exit", false, "exit 0 in case of error. NOTE: this flag will be removed once Chainloop reaches 1.0")
	cmd.PersistentFlags().StringVar(&attestationLocalStatePath, "local-state-path", "", "path to store the attestation state locally, default: [tmpDir]/chainloop_attestation.tmp.json")

	cmd.AddCommand(newAttestationInitCmd(), newAttestationAddCmd(), newAttestationStatusCmd(), newAttestationPushCmd(),
		newAttestationResetCmd(), newAttestationVerifyCmd())

	return cmd
}

func organizationExplicitlySelected(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	orgFlag := cmd.Root().PersistentFlags().Lookup(confOptions.organization.flagName)
	return (orgFlag != nil && orgFlag.Changed) || os.Getenv(CalculateEnvVarName(confOptions.organization.viperKey)) != ""
}

func resolveAttestationOrganization(cmd *cobra.Command) error {
	if organizationExplicitlySelected(cmd) {
		return nil
	}

	if repositoryOrganizationCommands[cmd.Name()] {
		cfg, path, err := repositoryconfig.LoadChainloopYML(".")
		if err == nil && cfg.Organization != "" {
			authToken, _, err := loadAuthToken(cmd)
			if err != nil {
				return err
			}
			if tokenOrg := internaltoken.MismatchedOrganization(authToken, cfg.Organization); tokenOrg != "" {
				return fmt.Errorf("credentials belong to organization %q but %q is configured in %s: use an API token for %q or remove organization from %s", tokenOrg, cfg.Organization, path, cfg.Organization, path)
			}

			setAttestationOrganization(cmd, cfg.Organization, path)
			if saved := viper.GetString(confOptions.organization.viperKey); saved != cfg.Organization {
				logger.Info().Str("organization", cfg.Organization).Str("file", path).Msg("using organization from repository config")
			}
			return nil
		}
		if err != nil {
			logger.Debug().Err(err).Msg("failed to load repository organization")
		}
	}

	// Preserve the existing local-state fallback for commands other than init.
	if cmd.Name() != "init" && !useAttestationRemoteState {
		if org := orgFromLocalState(attestationLocalStatePath); org != "" {
			setAttestationOrganization(cmd, org, "")
		}
	}

	return nil
}

func setAttestationOrganization(cmd *cobra.Command, organization, path string) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[attestationOrganizationNameAnnotation] = organization
	if path != "" {
		cmd.Annotations[attestationOrganizationFileAnnotation] = path
	}
}

func attestationOrganization(cmd *cobra.Command) (organization, path string) {
	if cmd == nil {
		return "", ""
	}
	return cmd.Annotations[attestationOrganizationNameAnnotation], cmd.Annotations[attestationOrganizationFileAnnotation]
}

func flagAttestationID(cmd *cobra.Command) {
	cmd.Flags().StringVar(&attestationID, "attestation-id", "", "Unique identifier of the in-progress attestation")
}

// orgFromLocalState reads the organization from the local attestation state file.
// Returns empty string on any error (file not found, parse error, etc.).
func orgFromLocalState(customPath string) string {
	raw, err := os.ReadFile(action.AttestationStatePath(customPath))
	if err != nil {
		return ""
	}

	state := &crafter.VersionedCraftingState{CraftingState: &v1.CraftingState{}}
	if err := protojson.Unmarshal(raw, state); err != nil {
		return ""
	}

	return state.GetAttestation().GetWorkflow().GetOrganization()
}

// extractAnnotations extracts the annotations from the flag and returns a map
// the expected input format is key=value
func extractAnnotations(annotationsFlag []string) (map[string]string, error) {
	var annotations = make(map[string]string)
	for _, annotation := range annotationsFlag {
		kv := strings.SplitN(annotation, "=", 2)
		if len(kv) < 2 {
			return nil, fmt.Errorf("invalid annotation %q, the format must be key=value", annotation)
		}
		annotations[kv[0]] = kv[1]
	}

	return annotations, nil
}
