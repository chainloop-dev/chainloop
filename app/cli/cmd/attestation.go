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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	robotAccount              string
	useAttestationRemoteState bool
	GracefulExit              bool
	// attestationID is the unique identifier of the in-progress attestation
	// this is required when use-attestation-remote-state is enabled
	attestationID string
)

const robotAccountEnvVarName = "CHAINLOOP_ROBOT_ACCOUNT"

func newAttestationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attestation",
		Aliases: []string{"att"},
		Short:   "Craft Software Supply Chain Attestations",
		Example: "Refer to https://docs.chainloop.dev/getting-started/attestation-crafting",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

	cmd.PersistentFlags().StringVarP(&robotAccount, "token", "t", "", fmt.Sprintf("robot account token. NOTE: You can also use the env variable %s", robotAccountEnvVarName))
	// We do not use viper in this case because we do not want this token to be saved in the config file
	// Instead we load the env variable manually
	if robotAccount == "" {
		robotAccount = os.Getenv(robotAccountEnvVarName)
	}

	cmd.PersistentFlags().BoolVar(&GracefulExit, "graceful-exit", false, "exit 0 in case of error. NOTE: this flag will be removed once Chainloop reaches 1.0")
	cmd.PersistentFlags().BoolVar(&useAttestationRemoteState, "remote-state", false, "Store the attestation state remotely (preview feature)")

	cmd.AddCommand(newAttestationInitCmd(), newAttestationAddCmd(), newAttestationStatusCmd(), newAttestationPushCmd(), newAttestationResetCmd())

	return cmd
}

func flagAttestationID(cmd *cobra.Command) {
	cmd.Flags().StringVar(&attestationID, "attestation-id", "", "Unique identifier of the in-progress attestation")
}

// extractAnnotations extracts the annotations from the flag and returns a map
// the expected input format is key=value
func extractAnnotations(annotationsFlag []string) (map[string]string, error) {
	var annotations = make(map[string]string)
	for _, annotation := range annotationsFlag {
		kv := strings.Split(annotation, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid annotation %q, the format must be key=value", annotation)
		}
		annotations[kv[0]] = kv[1]
	}

	return annotations, nil
}
