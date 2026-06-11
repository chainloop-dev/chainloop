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
	"fmt"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// eventPublisher is the subset of auditor.AuditLogPublisher used by the dispatcher
type eventPublisher interface {
	Publish(data *auditor.EventPayload) error
}

// AuditDispatcher publishes CAS audit events. Unlike the control plane's
// biz.AuditorUseCase, the actor is always SYSTEM (CAS JWTs carry no user
// identity) and the org comes from the JWT claims instead of the request context.
type AuditDispatcher struct {
	// nil when NATS is not configured, making the dispatcher a no-op
	publisher eventPublisher
	log       *log.Helper
}

func NewAuditDispatcher(publisher *auditor.AuditLogPublisher, logger log.Logger) *AuditDispatcher {
	d := &AuditDispatcher{log: servicelogger.ScopedHelper(logger, "audit-dispatcher")}
	// keep the interface nil when the publisher is disabled so shouldEmit short-circuits
	if publisher != nil {
		d.publisher = publisher
	}

	return d
}

// shouldEmit returns true when Dispatch would actually publish an event for the
// given claims. Hooks use it to skip extra work (e.g. backend Describe round-trips).
func (d *AuditDispatcher) shouldEmit(claims *casJWT.Claims) bool {
	return d != nil && d.publisher != nil && claims != nil && !claims.SourceInternal
}

// Dispatch generates and publishes an audit event with a SYSTEM actor and the
// organization from the JWT claims. Best-effort: errors are logged, never returned.
// Internal control plane traffic (SourceInternal claim) emits no events.
func (d *AuditDispatcher) Dispatch(entry auditor.LogEntry, claims *casJWT.Claims) {
	if !d.shouldEmit(claims) {
		return
	}

	orgID, err := uuid.Parse(claims.OrgID)
	if err != nil {
		d.log.Warnw("msg", "skipping audit event, invalid org id", "org_id", claims.OrgID, "error", err)
		return
	}

	payload, err := auditor.GenerateAuditEvent(entry,
		auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", ""),
		auditor.WithOrgID(orgID),
	)
	if err != nil {
		d.log.Errorw("msg", "failed to generate audit event", "error", err)
		sentry.CaptureException(fmt.Errorf("failed to generate audit event: %w", err))
		return
	}

	if err := d.publisher.Publish(payload); err != nil {
		d.log.Errorw("msg", "failed to publish audit event", "error", err)
		sentry.CaptureException(fmt.Errorf("failed to publish audit event: %w", err))
	}
}
