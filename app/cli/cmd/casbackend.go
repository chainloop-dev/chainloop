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

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
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
	}

	cmd.PersistentFlags().Bool("default", false, "set the backend as default in your organization")
	cmd.PersistentFlags().String("description", "", "descriptive information for this registration")
	cmd.PersistentFlags().String("name", "", "CAS backend name")
	err := cmd.MarkPersistentFlagRequired("name")
	cobra.CheckErr(err)

	cmd.AddCommand(newCASBackendAddOCICmd(), newCASBackendAddAzureBlobStorageCmd(), newCASBackendAddAWSS3Cmd())
	return cmd
}

func newCASBackendUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a CAS backend description, credentials or default status",
	}

	cmd.PersistentFlags().Bool("default", false, "set the backend as default in your organization")
	cmd.PersistentFlags().String("description", "", "descriptive information for this registration")

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
func confirmDefaultCASBackendRemoval(actionOpts *action.ActionsOpts, id string) (bool, error) {
	return confirmDefaultCASBackendUnset(id, "You are deleting the default CAS backend.", actionOpts)
}

func confirmDefaultCASBackendUnset(id, msg string, actionOpts *action.ActionsOpts) (bool, error) {
	// get existing backends
	backends, err := action.NewCASBackendList(actionOpts).Run()
	if err != nil {
		return false, fmt.Errorf("failed to list existing CAS backends: %w", err)
	}

	for _, b := range backends {
		// We are removing ourselves as the default, ask the user to confirm
		if b.Default && b.ID == id {
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
