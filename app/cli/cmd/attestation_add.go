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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

const NotSet = "[NOT SET]"

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
					// Try to load the value from a file or URL
					// If the value is a URL, it will be downloaded and stored in a temporary file
					// otherwise, it will be used as is
					var (
						rawValuePath string
						err          error
					)
					rawValuePath, err = getPathForResource(value)
					if err != nil {
						// If the error is an unrecognized scheme error, it means the path is not a URL
						// and we should take the value as is
						var uerr *unrecognizedSchemeError
						if errors.As(err, &uerr) {
							rawValuePath = value
						} else {
							return fmt.Errorf("loading resource: %w", err)
						}
					}
					// TODO: take the material output and show render it
					resp, err := a.Run(cmd.Context(), attestationID, name, rawValuePath, kind, annotations)
					if err != nil {
						return err
					}

					logger.Info().Msg("material added to attestation")

					policies, err := a.GetPolicyEvaluations(cmd.Context(), attestationID)
					if err != nil {
						return err
					}

					return encodeOutput(resp, func(s *action.AttestationStatusMaterial) error {
						return displayMaterialInfo(s, policies[resp.Name])
					})
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

// displayMaterialInfo prints the material information in a table format.
func displayMaterialInfo(status *action.AttestationStatusMaterial, policyEvaluations []*action.PolicyEvaluation) error {
	if status == nil {
		return nil
	}

	mt := newTableWriter()

	mt.AppendRow(table.Row{"Name", status.Material.Name})
	mt.AppendRow(table.Row{"Type", status.Material.Type})
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
				value = NotSet
			}
			mt.AppendRow(table.Row{"", fmt.Sprintf("%s: %s", a.Name, value)})
		}
	}

	if len(policyEvaluations) > 0 {
		mt.AppendRow(table.Row{"Policy evaluations", "------"})
	}

	policiesTable(policyEvaluations, mt)
	mt.SetStyle(table.StyleLight)
	mt.Style().Options.SeparateRows = true
	mt.Render()
	return nil
}

// unrecognizedSchemeError is an error type for when a URL scheme is not recognized out of
// the supported ones.
type unrecognizedSchemeError struct {
	Scheme string
}

func (e *unrecognizedSchemeError) Error() string {
	return fmt.Sprintf("loading URL: unrecognized scheme: %s", e.Scheme)
}

// getPathForResource tries to load a file or URL from the given path.
// If the path starts with "http://" or "https://", it will try to load the file from the URL and save it
// in a temporary file. It will return the path to the temporary file.
// If the path is an actual file path, it will return the filepath
func getPathForResource(resourcePath string) (string, error) {
	if _, err := os.Stat(resourcePath); err == nil {
		return resourcePath, nil
	}

	// Try to load the resource from a URL
	raw, err := loadResourceFromURLOrEnv(resourcePath)
	if err != nil {
		return "", fmt.Errorf("loading resource: %w", err)
	}

	// If the resource is loaded from a URL, save it in a temporary file
	return createTempFile(resourcePath, raw)
}

func loadResourceFromURLOrEnv(resourcePath string) ([]byte, error) {
	parts := strings.SplitAfterN(resourcePath, "://", 2)
	// If the path does not contain a scheme, it is considered a file path
	if len(parts) != 2 {
		return nil, &unrecognizedSchemeError{Scheme: parts[0]}
	}

	switch parts[0] {
	case "http://", "https://":
		return loadFromURL(resourcePath)
	case "env://":
		return loadFromEnv(parts[1])
	default:
		return nil, &unrecognizedSchemeError{Scheme: parts[0]}
	}
}

// loadFromURL loads the content of a URL and returns it as a byte slice.
func loadFromURL(url string) ([]byte, error) {
	// As cosign does: https://github.com/sigstore/cosign/blob/beb9cf21bc6741bc6e6b9736bdf57abfb91599c0/pkg/blob/load.go#L47
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("requesting URL: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("loading URL response: %w", err)
	}
	return raw, nil
}

// loadFromEnv loads the content of an environment variable and returns it as a byte slice.
func loadFromEnv(envVar string) ([]byte, error) {
	value, found := os.LookupEnv(envVar)
	if !found {
		return nil, fmt.Errorf("loading URL: env var $%s not found", envVar)
	}
	return []byte(value), nil
}

// createTempFile creates a temporary file with the given filename and writes the given data to it.
func createTempFile(filename string, rawData []byte) (string, error) {
	// Create a temporary directory with a random name to avoid collisions
	tempDir, err := os.MkdirTemp("", "chainloop-inflight-dir-*")
	if err != nil {
		return "", fmt.Errorf("creating temporary directory: %w", err)
	}

	// Create a temporary file with the same name as the original file
	tempFile, err := os.Create(filepath.Join(tempDir, filepath.Base(filename)))
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}

	// Write the data to the temporary file
	if _, err := tempFile.Write(rawData); err != nil {
		return "", fmt.Errorf("writing to temporary file: %w", err)
	}

	return tempFile.Name(), nil
}
