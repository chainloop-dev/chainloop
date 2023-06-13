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
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/spf13/cobra"
)

func newRegisteredIntegrationAddCmd() *cobra.Command {
	var options []string
	var integrationDescription string

	cmd := &cobra.Command{
		Use:     "add INTEGRATION_ID --options key=value,key=value",
		Short:   "Register a new instance of an integration",
		Example: `  chainloop integration registered add dependencytrack --options instance=https://deptrack.company.com,apiKey=1234567890`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := parseKeyValOpts(options)
			if err != nil {
				return err
			}

			res, err := action.NewRegisteredIntegrationAdd(actionOpts).Run(args[0], integrationDescription, opts)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.RegisteredIntegrationItem{res}, registeredIntegrationListTableOutput)
		},
	}

	cmd.Flags().StringVar(&integrationDescription, "description", "", "integration registration description")
	cmd.Flags().StringSliceVar(&options, "options", nil, "integration arguments")

	// We maintain the dependencytrack integration as a separate command for now
	// for compatibility reasons
	cmd.AddCommand(newRegisteredIntegrationAddDepTrackCmd())

	return cmd
}

func parseKeyValOpts(opts []string) (map[string]any, error) {
	var options = make(map[string]any)
	for _, opt := range opts {
		kv := strings.Split(opt, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid option %q, the expected format is key=value", opt)
		}
		options[kv[0]] = kv[1]
	}
	return options, nil
}
