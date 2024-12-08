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

package biz

import (
	"context"
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type AuditorUseCase struct {
	log       *log.Helper
	publisher *auditor.AuditLogPublisher
}

func NewAuditorUseCase(p *auditor.AuditLogPublisher, logger log.Logger) *AuditorUseCase {
	return &AuditorUseCase{
		log:       log.NewHelper(log.With(logger, "component", "biz/auditor")),
		publisher: p,
	}
}

// Dispatch logs an entry to the audit log asynchronously.
func (uc *AuditorUseCase) Dispatch(_ context.Context, entry auditor.LogEntry, orgID *uuid.UUID) {
	// TODO: extract user information
	// dynamically load user information from the context
	// user := entities.CurrentUser(ctx)
	// reset context so it can run in background
	opts := []auditor.GeneratorOption{}
	// if user != nil {
	// 	var actorType auditor.ActorType
	// 	if user.ServiceAccount {
	// 		actorType = auditor.ActorTypeAPIToken
	// 	} else {
	// 		actorType = auditor.ActorTypeUser
	// 	}

	// 	opts = append(opts, auditor.WithActor(actorType, user.ID, user.Email))
	// } else {
	// 	opts = append(opts, auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, ""))
	// }

	if orgID != nil {
		opts = append(opts, auditor.WithOrgID(*orgID))
	}

	payload, err := auditor.GenerateAuditEvent(entry, opts...)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("failed to generate audit event: %w", err))
		return
	}

	// Send event o event bus
	if err := uc.publisher.Publish(payload); err != nil {
		sentry.CaptureException(fmt.Errorf("failed to publish event: %w", err))
	}
}
