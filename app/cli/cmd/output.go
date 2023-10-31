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
	"io"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/jedib0t/go-pretty/v6/table"
)

const formatJSON = "json"
const formatTable = "table"

// Supported list of tabulated data that can be rendered as a table
type tabulatedData interface {
	[]*action.WorkflowItem |
		*action.AttestationStatusResult |
		[]*action.WorkflowRobotAccountItem |
		[]*action.WorkflowRunItem |
		*action.WorkflowRunItemFull |
		[]*action.WorkflowContractItem |
		*action.WorkflowContractWithVersionItem |
		*action.ConfigContextItem |
		[]*action.RegisteredIntegrationItem |
		[]*action.AvailableIntegrationItem |
		[]*action.AttachedIntegrationItem |
		[]*action.MembershipItem |
		[]*action.CASBackendItem |
		[]*action.OrgInviteItem
}

var ErrOutputFormatNotImplemented = errors.New("format not implemented")

// returns either json or table representation of the result
func encodeOutput[messageType tabulatedData, f func(messageType) error](v messageType, tableWriter f) error {
	switch flagOutputFormat {
	case formatJSON:
		return encodeJSON(v)
	case formatTable:
		return tableWriter(v)
	default:
		return ErrOutputFormatNotImplemented
	}
}

func encodeJSON(v interface{}) error {
	return encodeJSONToWriter(v, os.Stdout)
}

func encodeJSONToWriter(v interface{}, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "   ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}

	return nil
}

func newTableWriter() table.Writer {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.SetOutputMirror(os.Stdout)
	return tw
}
