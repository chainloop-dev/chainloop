//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newConfigSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "save",
		Short:   "Persist the current settings to the config file",
		Example: "chainloop config save --control-plane localhost:1234 --artifact-cas localhost:1235",
		Annotations: map[string]string{
			skipActionOptsInit: trueString,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Process CA flags - read file contents and encode to base64 if needed
			if err := processCAFlag(confOptions.controlplaneCA); err != nil {
				return err
			}
			if err := processCAFlag(confOptions.CASCA); err != nil {
				return err
			}
			return viper.WriteConfig()
		},
	}

	return cmd
}
