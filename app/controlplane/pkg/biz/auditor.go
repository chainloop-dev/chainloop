//
// Copyright 2024-2026 The Chainloop Authors.
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
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var auditorTracer = otelx.Tracer("chainloop-controlplane", "biz/auditor")

type AuditorUseCase struct {
	log        *log.Helper
	dispatcher *auditor.Dispatcher
}

func NewAuditorUseCase(p *auditor.AuditLogPublisher, logger log.Logger) *AuditorUseCase {
	// keep the Publisher interface nil when the publisher is disabled so the
	// dispatcher short-circuits instead of holding a typed-nil interface
	var publisher auditor.Publisher
	if p != nil {
		publisher = p
	}

	return &AuditorUseCase{
		log:        log.NewHelper(log.With(logger, "component", "biz/auditor")),
		dispatcher: auditor.NewDispatcher(publisher, logger),
	}
}

// Dispatch logs an entry to the audit log asynchronously. The actor is resolved
// from the request context (user, API token or system); the generation and
// publishing is delegated to the shared auditor.Dispatcher.
func (uc *AuditorUseCase) Dispatch(ctx context.Context, entry auditor.LogEntry, orgID *uuid.UUID) {
	ctx, span := otelx.Start(ctx, auditorTracer, "AuditorUseCase.Dispatch")
	defer span.End()

	// dynamically load user information from the context
	var opts []auditor.GeneratorOption
	var gotActor bool

	// Determine actor from context
	switch {
	case entities.CurrentUser(ctx) != nil:
		user := entities.CurrentUser(ctx)
		parsedUUID, _ := uuid.Parse(user.ID)
		fullName := strings.TrimSpace(fmt.Sprintf("%s %s", user.FirstName, user.LastName))
		opts = append(opts, auditor.WithActor(auditor.ActorTypeUser, parsedUUID, user.Email, fullName))
		gotActor = true
	case entities.CurrentAPIToken(ctx) != nil:
		apiToken := entities.CurrentAPIToken(ctx)
		parsedUUID, _ := uuid.Parse(apiToken.ID)
		opts = append(opts, auditor.WithActor(auditor.ActorTypeAPIToken, parsedUUID, "", apiToken.Name))
		gotActor = true
	default:
		opts = append(opts, auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", ""))
	}

	if !gotActor && entry.RequiresActor() {
		uc.log.Warn("failed to get actor information, required by the audit log entry")
		return
	}

	if orgID != nil {
		opts = append(opts, auditor.WithOrgID(*orgID))
	}

	uc.dispatcher.Dispatch(entry, opts...)
}
