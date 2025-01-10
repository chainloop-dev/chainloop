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
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

func newAttestationAddCmd() *cobra.Command {
	var name, value, kind string
	var artifactCASConn *grpc.ClientConn
	var annotationsFlag []string

	// OCI registry credentials can be passed as flags or environment variables
	var registryServer, registryUsername, registryPassword string
	const (
		registryServerEnvVarName   = "CHAINLOOP_REGISTRY_SERVER"
		registryUsernameEnvVarName = "CHAINLOOP_REGISTRY_USERNAME"
		// nolint: gosec
		registryPasswordEnvVarName = "CHAINLOOP_REGISTRY_PASSWORD"
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "add a material to the attestation",
		Annotations: map[string]string{
			useAPIToken: "true",
		},
		Example: `  # Add a material to the attestation that is defined in the contract
  chainloop attestation add --name <material-name> --value <material-value>

  # Add a material to the attestation that is not defined in the contract but you know the kind
  chainloop attestation add --kind <material-kind> --value <material-value>

  # Add a material to the attestation without specifying neither kind nor name enables automatic detection
  chainloop attestation add --value <material-value>`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if name != "" && kind != "" {
				return fmt.Errorf("both --name and --kind cannot be set at the same time")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := action.NewAttestationAdd(
				&action.AttestationAddOpts{
					ActionsOpts:        actionOpts,
					CASURI:             viper.GetString(confOptions.CASAPI.viperKey),
					CASCAPath:          viper.GetString(confOptions.CASCA.viperKey),
					ConnectionInsecure: apiInsecure(),
					RegistryServer:     registryServer,
					RegistryUsername:   registryUsername,
					RegistryPassword:   registryPassword,
					LocalStatePath:     attestationLocalStatePath,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to load action: %w", err)
			}

			// Extract annotations
			annotations, err := extractAnnotations(annotationsFlag)
			if err != nil {
				return err
			}

			// In some cases, the attestation state is stored remotely. To control concurrency we use
			// optimistic locking. We retry the operation if the state has changed since we last read it.
			return runWithBackoffRetry(
				func() error {
					// TODO: take the material output and show render it
					resp, err := a.Run(cmd.Context(), attestationID, name, value, kind, annotations)
					if err != nil {
						return err
					}

					logger.Info().Msg("material added to attestation")

					policies, err := a.GetPolicyEvaluations(cmd.Context(), attestationID)
					if err != nil {
						return err
					}

					return printMaterialInfo(resp, policies)
				},
			)
		},

		PostRunE: func(cmd *cobra.Command, args []string) error {
			if artifactCASConn != nil {
				return artifactCASConn.Close()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "name of the material as shown in the contract")
	cmd.Flags().StringVar(&value, "value", "", "value to be recorded")
	err := cmd.MarkFlagRequired("value")
	cobra.CheckErr(err)
	cmd.Flags().StringSliceVar(&annotationsFlag, "annotation", nil, "additional annotation in the format of key=value")
	flagAttestationID(cmd)
	cmd.Flags().StringVar(&kind, "kind", "", fmt.Sprintf("kind of the material to be recorded: %q", schemaapi.ListAvailableMaterialKind()))

	// Optional OCI registry credentials
	cmd.Flags().StringVar(&registryServer, "registry-server", "", fmt.Sprintf("OCI repository server, ($%s)", registryServerEnvVarName))
	cmd.Flags().StringVar(&registryUsername, "registry-username", "", fmt.Sprintf("registry username, ($%s)", registryUsernameEnvVarName))
	cmd.Flags().StringVar(&registryPassword, "registry-password", "", fmt.Sprintf("registry password, ($%s)", registryPasswordEnvVarName))

	if registryServer == "" {
		registryServer = os.Getenv(registryServerEnvVarName)
	}

	if registryUsername == "" {
		registryUsername = os.Getenv(registryUsernameEnvVarName)
	}

	if registryPassword == "" {
		registryPassword = os.Getenv(registryPasswordEnvVarName)
	}

	return cmd
}

// printMaterialInfo prints the material information in a table format.
func printMaterialInfo(status *action.AttestationStatusMaterial, policyEvaluations map[string][]*action.PolicyEvaluation) error {
	if status == nil {
		return nil
	}

	mt := newTableWriter()

	mt.AppendRow(table.Row{"Name", status.Material.Name})
	mt.AppendRow(table.Row{"Type", status.Material.Type})
	// mt.AppendRow(table.Row{"Set", hBool(status.Set)})
	mt.AppendRow(table.Row{"Required", hBool(status.Required)})

	if status.IsOutput {
		mt.AppendRow(table.Row{"Is output", "Yes"})
	}

	if status.Value != "" {
		v := status.Value
		if status.Tag != "" {
			v = fmt.Sprintf("%s:%s", v, status.Tag)
		}
		mt.AppendRow(table.Row{"Value", wrap.String(v, 100)})
	}

	if status.Hash != "" {
		mt.AppendRow(table.Row{"Digest", status.Hash})
	}

	if len(status.Material.Annotations) > 0 {
		mt.AppendRow(table.Row{"Annotations", "------"})
		for _, a := range status.Material.Annotations {
			value := a.Value
			if value == "" {
				value = "[NOT SET]"
			}
			mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	if len(policyEvaluations) > 0 {
		mt.AppendRow(table.Row{"Policies", "------"})
		for _, evaluations := range policyEvaluations {
			for _, ev := range evaluations {
				if len(ev.Violations) > 0 {
					for _, v := range ev.Violations {
						mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", ev.Name, v.Message)})
					}
				}
			}
		}
	}

	mt.SetStyle(table.StyleLight)
	mt.Style().Options.SeparateRows = true
	mt.Render()
	return nil
}
