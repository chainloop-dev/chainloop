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

package v1_test

import (
	"testing"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowServiceCreateRequestWorkflowTemplateID(t *testing.T) {
	testCases := []struct {
		desc       string
		templateID string
		wantErr    bool
	}{
		{
			desc:       "empty is allowed, the template reference is optional",
			templateID: "",
		},
		{
			desc:       "a valid UUID is accepted",
			templateID: "1b4c6f8a-2d3e-4f5a-8b9c-0d1e2f3a4b5c",
		},
		{
			desc:       "a non-UUID string is rejected",
			templateID: "sast-scan",
			wantErr:    true,
		},
		{
			desc:       "a malformed UUID is rejected",
			templateID: "1b4c6f8a-2d3e-4f5a-8b9c",
			wantErr:    true,
		},
	}

	validator, err := protovalidate.New()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req := &v1.WorkflowServiceCreateRequest{
				Name:               "my-workflow",
				ProjectName:        "my-project",
				WorkflowTemplateId: tc.templateID,
			}

			err := validator.Validate(req)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "workflow_template_id")
				return
			}

			assert.NoError(t, err)
		})
	}
}
