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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/spf13/cobra"
)

func newRegisteredIntegrationAddCmd() *cobra.Command {
	var options []string
	var integrationDescription string

	cmd := &cobra.Command{
		Use:     "add INTEGRATION_ID --options key=value,key=value",
		Short:   "Register a new instance of an integration",
		Example: `  chainloop integration registered add dependencytrack --opt instance=https://deptrack.company.com,apiKey=1234567890 --opt username=chainloop`,
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

			// Parse and validate options
			opts, err := parseAndValidateOpts(options, item.Registration)
			if err != nil {
				// Show schema table if validation fails
				if err := renderSchemaTable("Available options", item.Registration.Properties); err != nil {
					return err
				}
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
	// StringSlice seems to struggle with comma-separated values such as p12 jsonKeys provided as passwords
	// So we need to use StringArrayVar instead
	cmd.Flags().StringArrayVar(&options, "opt", nil, "integration arguments")

	return cmd
}

func parseAndValidateOpts(opts []string, schema *action.JSONSchema) (map[string]any, error) {
	// Parse
	res, err := parseKeyValOpts(opts, schema.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}

	// Validate
	if err = schema.Parsed.Validate(res); err != nil {
		// If validation fails, print the schema table
		var validationError *jsonschema.ValidationError

		// Prepare error message
		if errors.As(err, &validationError) {
			validationErrors := validationError.BasicOutput().Errors
			return nil, errors.New(validationErrors[len(validationErrors)-1].Error)
		}
	}

	return res, nil
}

// parseKeyValOpts performs two steps
// 1 - Split the options into key/value pairs
// 2 - Cast the values to the expected type defined in the schema
func parseKeyValOpts(opts []string, propertiesMap sdk.SchemaPropertiesMap) (map[string]any, error) {
	// 1 - Split the options into key/value pairs
	var options = make(map[string]any)
	for _, opt := range opts {
		kv := strings.SplitN(opt, "=", 2)
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
