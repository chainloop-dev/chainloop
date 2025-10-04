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

	"code.cloudfoundry.org/bytefmt"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

var (
	isDefaultCASBackendUpdateOption   *bool
	descriptionCASBackendUpdateOption *string
	maxBytesCASBackendOption          string
	parsedMaxBytes                    *int64
)

func newCASBackendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cas-backend",
		Short: "Operations on Artifact CAS backends",
	}

	cmd.AddCommand(newCASBackendListCmd(), newCASBackendAddCmd(), newCASBackendUpdateCmd(), newCASBackendDeleteCmd())
	return cmd
}

func newCASBackendAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new Artifact CAS backend",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return parseMaxBytesOption()
		},
	}

	cmd.PersistentFlags().Bool("default", false, "set the backend as default in your organization")
	cmd.PersistentFlags().String("description", "", "descriptive information for this registration")
	cmd.PersistentFlags().String("name", "", "CAS backend name")
	cmd.PersistentFlags().StringVar(&maxBytesCASBackendOption, "max-bytes", "", "Maximum size for each blob stored in this backend (e.g., 100MB, 1GB)")
	err := cmd.MarkPersistentFlagRequired("name")
	cobra.CheckErr(err)

	cmd.AddCommand(newCASBackendAddOCICmd(), newCASBackendAddAzureBlobStorageCmd(), newCASBackendAddAWSS3Cmd())
	return cmd
}

func newCASBackendUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a CAS backend description, credentials, default status, or max bytes",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return parseMaxBytesOption()
		},
	}

	cmd.PersistentFlags().Bool("default", false, "set the backend as default in your organization")
	cmd.PersistentFlags().String("description", "", "descriptive information for this registration")
	cmd.PersistentFlags().String("name", "", "CAS backend name")
	cmd.PersistentFlags().StringVar(&maxBytesCASBackendOption, "max-bytes", "", "Maximum size for each blob stored in this backend (e.g., 100MB, 1GB). Note: not supported for inline backends.")

	cmd.AddCommand(newCASBackendUpdateOCICmd(), newCASBackendUpdateInlineCmd(), newCASBackendUpdateAzureBlobCmd(), newCASBackendUpdateAWSS3Cmd())
	return cmd
}

// confirmDefaultCASBackendOverride asks the user to confirm the override of the default CAS backend
// in the event that there is one already set and its not the same as the one we are setting
func confirmDefaultCASBackendOverride(actionOpts *action.ActionsOpts, id string) (bool, error) {
	// get existing backends
	backends, err := action.NewCASBackendList(actionOpts).Run()
	if err != nil {
		return false, fmt.Errorf("failed to list existing CAS backends: %w", err)
	}

	// Find the default
	var defaultB *action.CASBackendItem
	for _, b := range backends {
		if b.Default {
			defaultB = b
			break
		}
	}

	// If there is none or there is but it's the same as the one we are setting, we are ok
	if defaultB == nil || (id != "" && id == defaultB.ID) {
		return true, nil
	}

	// Ask the user to confirm the override
	return confirmationPrompt("You are changing the default CAS backend in your organization"), nil
}

// If we are removing the default we confirm too
func confirmDefaultCASBackendRemoval(actionOpts *action.ActionsOpts, name string) (bool, error) {
	return confirmDefaultCASBackendUnset(name, "You are deleting the default CAS backend.", actionOpts)
}

func confirmDefaultCASBackendUnset(name, msg string, actionOpts *action.ActionsOpts) (bool, error) {
	// get existing backends
	backends, err := action.NewCASBackendList(actionOpts).Run()
	if err != nil {
		return false, fmt.Errorf("failed to list existing CAS backends: %w", err)
	}

	for _, b := range backends {
		// We are removing ourselves as the default, ask the user to confirm
		if b.Default && b.Name == name {
			return confirmationPrompt(msg), nil
		}
	}

	return true, nil
}

// y/n confirmation prompt
func confirmationPrompt(msg string) bool {
	fmt.Printf("%s\nPlease confirm to continue y/N\n", msg)
	var gotChallenge string
	fmt.Scanln(&gotChallenge)

	return gotChallenge == "y" || gotChallenge == "Y"
}

// parseMaxBytesOption validates and parses the --max-bytes flag value.
// It stores the parsed result in parsedMaxBytes for child commands to use.
func parseMaxBytesOption() error {
	parsedMaxBytes = nil
	if maxBytesCASBackendOption == "" {
		return nil
	}

	bytes, err := bytefmt.ToBytes(maxBytesCASBackendOption)
	if err != nil {
		return fmt.Errorf("invalid max-bytes format: %w", err)
	}

	bytesInt := int64(bytes)
	parsedMaxBytes = &bytesInt
	return nil
}

// captureUpdateFlags reads the --default and --description flags only when explicitly set and
// stores their values in the package-level pointer options. This avoids treating their zero
// values as an intention to update.
func captureUpdateFlags(cmd *cobra.Command) error {
	if f := cmd.Flags().Lookup("default"); f != nil && f.Changed {
		v, err := cmd.Flags().GetBool("default")
		if err != nil {
			return err
		}
		isDefaultCASBackendUpdateOption = &v
	}

	if f := cmd.Flags().Lookup("description"); f != nil && f.Changed {
		v, err := cmd.Flags().GetString("description")
		if err != nil {
			return err
		}
		descriptionCASBackendUpdateOption = &v
	}

	return nil
}

// handleDefaultUpdateConfirmation centralizes the confirmation logic when the --default flag
// is provided. It returns (true, nil) when it's ok to proceed, (false, nil) when the user
// declined confirmation, or (false, err) when an error happened.
func handleDefaultUpdateConfirmation(actionOpts *action.ActionsOpts, name string) (bool, error) {
	if isDefaultCASBackendUpdateOption == nil {
		return true, nil
	}

	if *isDefaultCASBackendUpdateOption {
		return confirmDefaultCASBackendOverride(actionOpts, name)
	}

	return confirmDefaultCASBackendUnset(name, "You are setting the default CAS backend to false", actionOpts)
}
