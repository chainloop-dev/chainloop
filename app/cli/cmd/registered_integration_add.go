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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/spf13/cobra"
)

func newRegisteredIntegrationAddCmd() *cobra.Command {
	var flagOpts []string
	var integrationDescription string

	cmd := &cobra.Command{
		Use:     "add INTEGRATION_ID --options key=value,key=value",
		Short:   "Register a new instance of an integration",
		Example: `  chainloop integration registered add dependencytrack --options instance=https://deptrack.company.com,apiKey=1234567890`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Retrieve schema for validation and options marshaling
			item, err := action.NewAvailableIntegrationDescribe(actionOpts).Run(args[0])
			if err != nil {
				return err
			}

			if item == nil {
				return fmt.Errorf("integration %q not found", args[0])
			}

			// Parse options
			opts, err := parseKeyValOpts(flagOpts, item.Registration.Properties)
			if err != nil {
				if err := renderSchemaTable("Available options", item.Registration.Properties); err != nil {
					return err
				}
				return err
			}

			// Validate options against schema
			if err = validateAgainstSchema(opts, item.Registration.Parsed); err != nil {
				// If validation fails, print the schema table
				var validationError *jsonschema.ValidationError

				if errors.As(err, &validationError) {
					if err := renderSchemaTable("Available options", item.Registration.Properties); err != nil {
						return err
					}
				}

				validationErrors := validationError.BasicOutput().Errors
				return errors.New(validationErrors[len(validationErrors)-1].Error)
			}

			res, err := action.NewRegisteredIntegrationAdd(actionOpts).Run(args[0], integrationDescription, opts)
			if err != nil {
				return err
			}

			return encodeOutput([]*action.RegisteredIntegrationItem{res}, registeredIntegrationListTableOutput)
		},
	}

	cmd.Flags().StringVar(&integrationDescription, "description", "", "integration registration description")
	cmd.Flags().StringSliceVar(&flagOpts, "options", nil, "integration arguments")

	// We maintain the dependencytrack integration as a separate command for now
	// for compatibility reasons
	cmd.AddCommand(newRegisteredIntegrationAddDepTrackCmd())

	return cmd
}

func validateAgainstSchema(opts map[string]any, schema *jsonschema.Schema) error {
	// 1 - Marshal the options to JSON
	b, err := json.Marshal(opts)
	if err != nil {
		return err
	}

	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// 2 - Validate the JSON against the schema
	err = schema.Validate(v)
	if err != nil {
		return err
	}

	return nil
}

func parseKeyValOpts(opts []string, propertiesMap action.SchemaPropertiesMap) (map[string]any, error) {
	// Two steps process

	// 1 - Split the options into key/value pairs
	var options = make(map[string]any)
	for _, opt := range opts {
		kv := strings.Split(opt, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid option %q, the expected format is key=value", opt)
		}
		options[kv[0]] = kv[1]
	}

	// 2 - Cast the values to the expected type defined in the schema
	for k, v := range options {
		prop, ok := propertiesMap[k]
		if !ok {
			continue
		}

		switch prop.Type {
		case "string":
			options[k] = v.(string)
		case "integer":
			nv, err := strconv.Atoi(v.(string))
			if err != nil {
				return nil, fmt.Errorf("invalid option %q, the expected format is %q", v, prop.Type)
			}

			options[k] = nv
		case "number":
			nv, err := strconv.ParseFloat(v.(string), 32)
			if err != nil {
				return nil, fmt.Errorf("invalid option %q, the expected format is %q", v, prop.Type)
			}

			options[k] = nv
		case "boolean":
			nv, err := strconv.ParseBool(v.(string))
			if err != nil {
				return nil, fmt.Errorf("invalid option %q, the expected format is %q", v, prop.Type)
			}
			options[k] = nv
		}
	}

	return options, nil
}
