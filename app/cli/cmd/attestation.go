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

	"github.com/spf13/cobra"
)

var (
	robotAccount string
	GracefulExit bool
)

const robotAccountEnvVarName = "CHAINLOOP_ROBOT_ACCOUNT"

func newAttestationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attestation",
		Aliases: []string{"att"},
		Short:   "Craft Software Supply Chain Attestations",
		Example: "Refer to https://docs.chainloop.dev/getting-started/attestation-crafting",
	}

	cmd.PersistentFlags().StringVarP(&robotAccount, "token", "t", "", fmt.Sprintf("robot account token. NOTE: You can also use the env variable %s", robotAccountEnvVarName))
	// We do not use viper in this case because we do not want this token to be saved in the config file
	// Instead we load the env variable manually
	if robotAccount == "" {
		robotAccount = os.Getenv(robotAccountEnvVarName)
	}
	cmd.PersistentFlags().BoolVar(&GracefulExit, "graceful-exit", false, "exit 0 in case of error. NOTE: this flag will be removed once Chainloop reaches 1.0")

	cmd.AddCommand(newAttestationInitCmd(), newAttestationAddCmd(), newAttestationStatusCmd(), newAttestationPushCmd(), newAttestationResetCmd())

	return cmd
}
