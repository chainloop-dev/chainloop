//
// Copyright 2024 The Chainloop Authors.
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
	"sort"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

const formatContract = "schema"

func newWorkflowContractDescribeCmd() *cobra.Command {
	var name string
	var revision int32

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe the information of the contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowContractDescribe(actionOpts).Run(name, revision)
			if err != nil {
				return err
			}

			return encodeContractOutput(res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "contract name")
	err := cmd.MarkFlagRequired("name")
	cobra.CheckErr(err)

	// Override default output flag
	cmd.InheritedFlags().StringVarP(&flagOutputFormat, "output", "o", "table", "output format, valid options are table, json or schema")

	cmd.Flags().Int32Var(&revision, "revision", 0, "revision of the contract to retrieve, by default is latest")

	return cmd
}

func encodeContractOutput(run *action.WorkflowContractWithVersionItem) error {
	if flagOutputFormat != formatContract {
		logger.Info().Msg("To download the contract, run the command with the \"--output schema\" option")
	}

	err := encodeOutput(run, contractDescribeTableOutput)
	if err == nil || !errors.Is(err, ErrOutputFormatNotImplemented) {
		return err
	}

	switch flagOutputFormat {
	case formatContract:
		fmt.Fprintln(os.Stdout, run.Revision.RawBody.Body)
		return nil
	default:
		return ErrOutputFormatNotImplemented
	}
}

func contractDescribeTableOutput(contractWithVersion *action.WorkflowContractWithVersionItem) error {
	revision := contractWithVersion.Revision

	c := contractWithVersion.Contract
	t := newTableWriter()
	t.SetTitle("Contract")
	t.AppendRow(table.Row{"Name", c.Name})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Description", c.Description})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Associated Workflows", stringifyAssociatedWorkflows(contractWithVersion)})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Revision number", revision.Revision})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Revision Created At", revision.CreatedAt.Format(time.RFC822)})
	t.Render()

	vt := newTableWriter()
	vt.AppendRow(table.Row{revision.RawBody.Body})
	vt.Render()

	return nil
}

// stringifyAssociatedWorkflows returns a string representation of the associated workflows by combining
// the project name the workflow belongs to and workflow name
func stringifyAssociatedWorkflows(contractWithRevision *action.WorkflowContractWithVersionItem) string {
	contract := contractWithRevision.Contract

	workflows := make([]string, 0, len(contract.WorkflowRefs))
	for _, w := range contract.WorkflowRefs {
		workflows = append(workflows, fmt.Sprintf("%s/%s", w.ProjectName, w.Name))
	}
	// sort the workflows
	sort.Strings(workflows)

	return strings.Join(workflows, "\n")
}
