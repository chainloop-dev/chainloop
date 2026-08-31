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

package service

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBizWorkflowToPbWorkflowTemplateID(t *testing.T) {
	createdAt := time.Now()
	templateID := uuid.MustParse("1b4c6f8a-2d3e-4f5a-8b9c-0d1e2f3a4b5c")

	testCases := []struct {
		desc       string
		templateID *uuid.UUID
		want       string
	}{
		{
			desc:       "unbound workflow maps to an empty template ID",
			templateID: nil,
			want:       "",
		},
		{
			desc:       "template-backed workflow maps to the template ID",
			templateID: &templateID,
			want:       templateID.String(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := bizWorkflowToPb(&biz.Workflow{
				ID:                 uuid.New(),
				Name:               "my-workflow",
				CreatedAt:          &createdAt,
				ProjectID:          uuid.New(),
				WorkflowTemplateID: tc.templateID,
			})

			assert.Equal(t, tc.want, got.GetWorkflowTemplateId())
		})
	}
}
