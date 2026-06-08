//
// Copyright 2026 The Chainloop Authors.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRunListPreRunValidation(t *testing.T) {
	testCases := []struct {
		name    string
		version string
		project string
		wantErr string
	}{
		{
			name:    "version without project is rejected",
			version: "v1.0.0",
			project: "",
			wantErr: "--project is required when --version is set",
		},
		{
			name:    "version with project is allowed",
			version: "v1.0.0",
			project: "my-project",
		},
		{
			name:    "no version is allowed without project",
			version: "",
			project: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newWorkflowWorkflowRunListCmd()
			require.NoError(t, cmd.Flags().Set("version", tc.version))
			require.NoError(t, cmd.Flags().Set("project", tc.project))

			err := cmd.PreRunE(cmd, nil)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
