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
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newAuthDeleteAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-account",
		Short: "delete your account",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get user information to make sure the user knows the account she is deleting
			contextResp, err := action.NewConfigCurrentContext(actionOpts).Run()
			if err != nil {
				return err
			}

			fmt.Printf("You are about to delete your account %q\n", contextResp.CurrentUser.Email)

			// Ask for confirmation
			if err := confirmDeletion(); err != nil {
				return err
			}

			// Account deletion
			if err := action.NewDeleteAccount(actionOpts).Run(); err != nil {
				return err
			}

			// Remove token from config file
			viper.Set(confOptions.authToken.viperKey, "")
			if err := viper.WriteConfig(); err != nil {
				return err
			}

			logger.Info().Msg("Account deleted :(")
			return nil
		},
	}

	return cmd
}

// confirmDeletion asks the user to type a random string
func confirmDeletion() error {
	wantChallenge := deletionChallenge()
	fmt.Printf("To confirm, please type %q\n", wantChallenge)

	var gotChallenge string
	fmt.Scanln(&gotChallenge)

	if gotChallenge != wantChallenge {
		return errors.New("confirmation code does not match")
	}

	return nil
}

// deletionChallenge generates a random string
func deletionChallenge() string {
	const n = 8 // desired length of the random string

	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	var result string

	for i := 0; i < n; i++ {
		result += string(letterBytes[int(b[i])%len(letterBytes)])
		if i == 3 {
			result += "-"
		}
	}

	return result
}
