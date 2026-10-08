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

package dispatcher

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWorkflowMetadata_UnfinishedRun pins the crash condition: a run that is
// not yet finished has a nil FinishedAt (and may have a nil Attestation). The
// dispatcher must build its metadata without dereferencing those nil pointers.
func TestNewWorkflowMetadata_UnfinishedRun(t *testing.T) {
	createdAt := time.Now()
	wfRun := &biz.WorkflowRun{
		ID:         uuid.New(),
		State:      "initialized",
		CreatedAt:  &createdAt,
		FinishedAt: nil, // unfinished run
		RunnerType: "GENERIC",
		RunURL:     "https://ci.example/run/1",
		// Attestation is nil until the bundle is stored.
	}
	wf := &biz.Workflow{Name: "wf", Project: "proj", Team: "team"}
	opts := &RunOpts{WorkflowID: uuid.NewString(), WorkflowRunID: wfRun.ID.String()}

	require.NotPanics(t, func() {
		got := newWorkflowMetadata(opts, wf, wfRun)
		require.NotNil(t, got)
		require.NotNil(t, got.WorkflowRun)

		assert.True(t, got.WorkflowRun.FinishedAt.IsZero(), "nil FinishedAt must map to the zero time")
		assert.Equal(t, createdAt, got.WorkflowRun.StartedAt)
		assert.Empty(t, got.WorkflowRun.AttestationDigest, "nil Attestation must map to an empty digest")
		assert.Equal(t, "wf", got.Workflow.Name)
		assert.Equal(t, "proj", got.Workflow.Project)
	})
}

// TestNewWorkflowMetadata_FinishedRun confirms the populated fields still flow
// through unchanged once the run is finalized.
func TestNewWorkflowMetadata_FinishedRun(t *testing.T) {
	createdAt := time.Now().Add(-time.Hour)
	finishedAt := time.Now()
	wfRun := &biz.WorkflowRun{
		ID:          uuid.New(),
		State:       "success",
		CreatedAt:   &createdAt,
		FinishedAt:  &finishedAt,
		RunnerType:  "GENERIC",
		RunURL:      "https://ci.example/run/1",
		Attestation: &biz.Attestation{Digest: "sha256:abc"},
	}
	wf := &biz.Workflow{Name: "wf", Project: "proj", Team: "team"}
	opts := &RunOpts{WorkflowID: uuid.NewString(), WorkflowRunID: wfRun.ID.String()}

	got := newWorkflowMetadata(opts, wf, wfRun)

	assert.Equal(t, createdAt, got.WorkflowRun.StartedAt)
	assert.Equal(t, finishedAt, got.WorkflowRun.FinishedAt)
	assert.Equal(t, "sha256:abc", got.WorkflowRun.AttestationDigest)
	assert.Equal(t, "success", got.WorkflowRun.State)
}
