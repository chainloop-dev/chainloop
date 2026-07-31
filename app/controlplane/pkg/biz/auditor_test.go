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

package biz

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPublisher captures the audit events a use case dispatches so tests
// can assert on them. The production publisher is NATS-backed, so this is the
// only way to observe dispatches without a broker.
type recordingPublisher struct {
	published []*auditor.EventPayload
}

func (p *recordingPublisher) Publish(data *auditor.EventPayload) error {
	p.published = append(p.published, data)
	return nil
}

// assertSingleProjectVersionEvent checks that exactly one event was recorded,
// that it reports the given action for the given project, and that its payload
// describes the given version. wantMarkedAsLatest pins whether the event
// announces a promotion.
func (p *recordingPublisher) assertSingleProjectVersionEvent(
	t *testing.T, wantAction string, project *Project, version *ProjectVersion, wantMarkedAsLatest bool,
) {
	t.Helper()

	require.Len(t, p.published, 1)
	got := p.published[0].Data
	assert.Equal(t, wantAction, got.ActionType)
	assert.Equal(t, &project.ID, got.TargetID)
	assert.Equal(t, &project.OrgID, got.OrgID)

	var info struct {
		VersionID      *uuid.UUID `json:"version_id"`
		Version        string     `json:"version"`
		MarkedAsLatest bool       `json:"marked_as_latest"`
	}
	require.NoError(t, json.Unmarshal(got.Info, &info))
	assert.Equal(t, &version.ID, info.VersionID)
	assert.Equal(t, version.Version, info.Version)
	assert.Equal(t, wantMarkedAsLatest, info.MarkedAsLatest)
}

// newRecordingAuditor builds an AuditorUseCase backed by a recordingPublisher,
// through the same constructor production uses so the two cannot drift.
func newRecordingAuditor() (*AuditorUseCase, *recordingPublisher) {
	publisher := &recordingPublisher{}

	return newAuditorUseCase(publisher, log.NewStdLogger(io.Discard)), publisher
}

// ctxWithAPITokenActor returns a context carrying an actor, required by the
// audit entries that report on project resources.
func ctxWithAPITokenActor(ctx context.Context) context.Context {
	return entities.WithCurrentAPIToken(ctx, &entities.APIToken{ID: uuid.NewString(), Name: "test-token"})
}
