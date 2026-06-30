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
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/bytefmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/resourceloader"
)

const NotSet = "[NOT SET]"

func newAttestationAddCmd() *cobra.Command {
	var name, value, kind string
	var artifactCASConn *grpc.ClientConn
	var annotationsFlag []string
	var noStrictValidation bool
	var policyInputFromFileFlag []string
	var maxExtractEntries int
	var maxExtractSize string

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
			useAPIToken:                     "true",
			supportsFederatedAuthAnnotation: "true",
		},
		Example: `  # Add a material to the attestation that is defined in the contract
  chainloop attestation add --name <material-name> --value <material-value>

  # Add a material to the attestation that is not defined in the contract but you know the kind
  chainloop attestation add --kind <material-kind> --value <material-value>

  # Add a material to the attestation without specifying neither kind nor name enables automatic detection
  chainloop attestation add --value <material-value>

  # Add a material by also providing a URL pointing to the material. It will be downloaded to a temporary folder first
  chainloop attestation add --value https://example.com/sbom.json

  # Feed a policy input from a column of a CSV/JSON file (e.g. the ignored_paths exclusion list for the sigcheck binary-signing policies).
  # The :column suffix selects the column; it defaults to the input name when omitted. The file is also recorded as EVIDENCE.
  chainloop attestation add --name sigcheck --value sigcheckResult.csv --kind SYSINTERNALS_SIGCHECK \
    --policy-input-from-file ignored_paths=exception.csv:Path

  # Scope an input to a specific policy with a <policy>: prefix so it only applies to that policy attachment.
  chainloop attestation add --name sigcheck --value sigcheckResult.csv --kind SYSINTERNALS_SIGCHECK \
    --policy-input-from-file trusted-binaries-signed:ignored_paths=exception.csv:Path \
    --policy-input-from-file trusted-binaries-vendor-keys:third_party_paths=exception.csv:Path`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			maxExtractSizeBytes, err := bytefmt.ToBytes(maxExtractSize)
			if err != nil {
				return fmt.Errorf("invalid --max-extract-size %q: %w", maxExtractSize, err)
			}

			a, err := action.NewAttestationAdd(
				&action.AttestationAddOpts{
					ActionsOpts:        ActionOpts,
					CASURI:             viper.GetString(confOptions.CASAPI.viperKey),
					CASCAPath:          viper.GetString(confOptions.CASCA.viperKey),
					ConnectionInsecure: apiInsecure(),
					RegistryServer:     registryServer,
					RegistryUsername:   registryUsername,
					RegistryPassword:   registryPassword,
					LocalStatePath:     attestationLocalStatePath,
					NoStrictValidation: noStrictValidation,
					MaxExtractEntries:  maxExtractEntries,
					MaxExtractSize:     int64(maxExtractSizeBytes),
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

			// Parse and resolve the policy input files (column -> policy input).
			// Done once here; the resolved local paths are reused across retries.
			policyInputFiles, err := resolvePolicyInputFiles(policyInputFromFileFlag)
			if err != nil {
				return err
			}

			// In some cases, the attestation state is stored remotely. To control concurrency we use
			// optimistic locking. We retry the operation if the state has changed since we last read it.
			return runWithBackoffRetry(
				func() error {
					// Try to load the value from a file or URL
					// If the value is a URL, it will be downloaded and stored in a temporary file
					// otherwise, it will be used as is
					rawValuePath, err := resourceloader.GetPathForResource(value)
					if err != nil {
						// If the error is an unrecognized scheme error, it means the path is not a URL
						// and we should take the value as is
						var uerr *resourceloader.UnrecognizedSchemeError
						if errors.As(err, &uerr) {
							rawValuePath = value
						} else {
							return fmt.Errorf("loading resource: %w", err)
						}
					}
					resp, err := a.Run(cmd.Context(), attestationID, name, rawValuePath, kind, annotations, policyInputFiles)
					if err != nil {
						return err
					}

					logger.Info().Int("materials", len(resp)).Msg("material(s) added to attestation")

					policies, err := a.GetPolicyEvaluations(cmd.Context(), attestationID)
					if err != nil {
						return err
					}

					// The explode path can return several materials. Render JSON as a
					// single array so the output stays a parseable document; only the
					// table renderer is emitted per material.
					switch flagOutputFormat {
					case output.FormatJSON:
						return output.EncodeJSON(resp)
					case output.FormatTable:
						for _, m := range resp {
							if err := displayMaterialInfo(m, policies[m.Name]); err != nil {
								return err
							}
						}
						return nil
					default:
						return output.ErrOutputFormatNotImplemented
					}
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
	cmd.Flags().BoolVar(&noStrictValidation, "no-strict-validation", false, "skip strict schema validation for structured materials (SBOM_CYCLONEDX_JSON, OPENAPI_SPEC, ASYNCAPI_SPEC, OSSF_SCORECARD_JSON)")
	cmd.Flags().StringArrayVar(&policyInputFromFileFlag, "policy-input-from-file", nil, "feed a policy input from a column of a CSV or JSON file, in the format [<policy>:]<input>=<file>[:<column>] (e.g. ignored_paths=exception.csv:Path); an optional <policy>: prefix scopes the input to a single policy (matched by name or ref), otherwise it applies to every declaring policy; <column> is a single top-level column/field name and defaults to the input name; repeatable. The file is also recorded as EVIDENCE.")

	// Optional OCI registry credentials
	cmd.Flags().StringVar(&registryServer, "registry-server", "", fmt.Sprintf("OCI repository server, ($%s)", registryServerEnvVarName))
	cmd.Flags().StringVar(&registryUsername, "registry-username", "", fmt.Sprintf("registry username, ($%s)", registryUsernameEnvVarName))
	cmd.Flags().StringVar(&registryPassword, "registry-password", "", fmt.Sprintf("registry password, ($%s)", registryPasswordEnvVarName))

	// Archive extraction guards
	cmd.Flags().IntVar(&maxExtractEntries, "max-extract-entries", 10000, "max number of files to extract when --value is an archive")
	cmd.Flags().StringVar(&maxExtractSize, "max-extract-size", "1GiB", "max total uncompressed size to extract when --value is an archive")

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

// resolvePolicyInputFiles parses each --policy-input-from-file value and
// resolves its file reference to a local path (downloading URLs to a temporary
// file, mirroring how --value is handled).
func resolvePolicyInputFiles(raw []string) ([]*action.PolicyInputFromFile, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	result := make([]*action.PolicyInputFromFile, 0, len(raw))
	for _, r := range raw {
		pif, err := action.ParsePolicyInputFromFile(r)
		if err != nil {
			return nil, err
		}

		path, err := resourceloader.GetPathForResource(pif.File)
		if err != nil {
			var uerr *resourceloader.UnrecognizedSchemeError
			if errors.As(err, &uerr) {
				path = pif.File
			} else {
				return nil, fmt.Errorf("loading policy input file: %w", err)
			}
		}
		pif.File = path

		result = append(result, pif)
	}

	return result, nil
}

// displayMaterialInfo prints the material information in a table format.
func displayMaterialInfo(status *action.AttestationStatusMaterial, policyEvaluations []*action.PolicyEvaluation) error {
	if status == nil {
		return nil
	}

	mt := output.NewTableWriter()

	mt.AppendRow(table.Row{"Name", status.Name})
	mt.AppendRow(table.Row{"Type", status.Type})
	mt.AppendRow(table.Row{"Required", hBool(status.Required)})

	if status.Group != "" {
		mt.AppendRow(table.Row{"Group", status.Group})
		mt.AppendRow(table.Row{"Rule", "at least one of the group required"})
	}

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

	if len(status.Annotations) > 0 {
		mt.AppendRow(table.Row{"Annotations", "------"})
		for _, a := range status.Annotations {
			value := a.Value
			if value == "" {
				value = NotSet
			}
			mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	if len(policyEvaluations) > 0 {
		mt.AppendRow(table.Row{"Policy evaluations", "------"})
	}

	policiesTable(policyEvaluations, mt, flagDebug)
	mt.SetStyle(table.StyleLight)
	mt.Style().Options.SeparateRows = true
	mt.Render()
	return nil
}
