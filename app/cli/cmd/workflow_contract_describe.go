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
	"os"
	"strings"
	"time"

	"github.com/chainloop-dev/bedrock/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

const formatContract = "schema"

func newWorkflowContractDescribeCmd() *cobra.Command {
	var contractID string
	var revision int32

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe the information of the contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := action.NewWorkflowContractDescribe(actionOpts).Run(contractID, revision)
			if err != nil {
				return err
			}

			return encodeContractOutput(res)
		},
	}

	cmd.Flags().StringVar(&contractID, "id", "", "contract ID")
	err := cmd.MarkFlagRequired("id")
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
		marshaller := protojson.MarshalOptions{Indent: "  "}
		rawBody, err := marshaller.Marshal(run.Revision.BodyV1)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(rawBody))

		return nil
	default:
		return ErrOutputFormatNotImplemented
	}
}

func contractDescribeTableOutput(contractWithVersion *action.WorkflowContractWithVersionItem) error {
	revision := contractWithVersion.Revision

	marshaller := protojson.MarshalOptions{Indent: "  "}
	rawBody, err := marshaller.Marshal(revision.BodyV1)
	if err != nil {
		return err
	}

	c := contractWithVersion.Contract
	t := newTableWriter()
	t.SetTitle("Contract")
	t.AppendRow(table.Row{"Name", c.Name})
	t.AppendSeparator()
	t.AppendRow(table.Row{"ID", c.ID})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Associated Workflows", strings.Join(c.WorkflowIDs, ", ")})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Revision number", revision.Revision})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Revision Created At", revision.CreatedAt.Format(time.RFC822)})
	t.Render()

	vt := newTableWriter()
	vt.AppendRow(table.Row{string(rawBody)})
	vt.Render()

	return nil
}
