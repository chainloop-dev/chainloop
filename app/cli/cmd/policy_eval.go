//
// Copyright 2025 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/output"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/spf13/cobra"
)

func newPolicyEvalCmd() *cobra.Command {
	var (
		materialPath string
		kind         string
		annotations  []string
		policyPath   string
		inputs       []string
	)

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate a policy",
		Long: `Evaluate a policy with organization settings.

This command uses organization context to evaluate policies.

For offline development and testing with debug capabilities, use 'chainloop policy develop eval' instead.`,
		Example: `
  chainloop policy eval --policy policy.yaml --input digest=sha256:80058e45a56daa50ae2a130bd1bd13b1fb9aff13a55b2d98615fff6eb3b0fffb`,
		Annotations: map[string]string{
			useAPIToken: trueString,
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := &action.PolicyEvaluateOpts{
				MaterialPath: materialPath,
				Kind:         kind,
				Annotations:  parseKeyValue(annotations),
				PolicyPath:   policyPath,
				Inputs:       parseKeyValue(inputs),
			}

			policyEval, err := action.NewPolicyEvaluate(opts, ActionOpts)
			if err != nil {
				return err
			}

			result, err := policyEval.Run(cmd.Context())
			if err != nil {
				return err
			}

			return output.EncodeJSON(result)
		},
	}

	cmd.Flags().StringVar(&materialPath, "material", "", "Path to material or attestation file")
	cmd.Flags().StringVar(&kind, "kind", "", fmt.Sprintf("Kind of the material: %q", schemaapi.ListAvailableMaterialKind()))
	cmd.Flags().StringSliceVar(&annotations, "annotation", []string{}, "Key-value pairs of material annotations (key=value)")
	cmd.Flags().StringVarP(&policyPath, "policy", "p", "", "Policy reference (./my-policy.yaml, https://my-domain.com/my-policy.yaml)")
	cobra.CheckErr(cmd.MarkFlagRequired("policy"))
	cmd.Flags().StringArrayVar(&inputs, "input", []string{}, "Key-value pairs of policy inputs (key=value)")

	return cmd
}
