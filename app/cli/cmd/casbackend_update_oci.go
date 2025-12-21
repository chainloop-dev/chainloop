//
// Copyright 2024-2025 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
)

func newCASBackendUpdateOCICmd() *cobra.Command {
	var backendName, username, password string
	cmd := &cobra.Command{
		Use:   "oci",
		Short: "Update a OCI CAS Backend description, credentials, default status, or max bytes",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return parseMaxBytesOption()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// capture flags only when explicitly set
			if err := captureUpdateFlags(cmd); err != nil {
				return err
			}

			// If we are overriding/unsetting the default we ask for confirmation
			if ok, err := handleDefaultUpdateConfirmation(ActionOpts, backendName); err != nil {
				return err
			} else if !ok {
				log.Info("Aborting...")
				return nil
			}

			opts := &action.NewCASBackendUpdateOpts{
				Name:        backendName,
				Description: descriptionCASBackendUpdateOption,
				Credentials: map[string]any{
					"username": username,
					"password": password,
				},
				Default:  isDefaultCASBackendUpdateOption,
				Fallback: isFallbackCASBackendUpdateOption,
				MaxBytes: parsedMaxBytes,
			}

			if username == "" && password == "" {
				opts.Credentials = nil
			}

			res, err := action.NewCASBackendUpdate(ActionOpts).Run(opts)
			if err != nil {
				return err
			} else if res == nil {
				return nil
			}

			return output.EncodeOutput(flagOutputFormat, res, casBackendItemTableOutput)
		},
	}

	cmd.Flags().StringVar(&backendName, "name", "", "CAS Backend name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	cmd.Flags().StringVarP(&username, "username", "u", "", "registry username")

	cmd.Flags().StringVarP(&password, "password", "p", "", "registry password")

	return cmd
}
