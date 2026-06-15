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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// AuditDispatcher publishes CAS audit events. It delegates the shared
// generate -> publish -> error-reporting flow to the control plane's
// auditor.Dispatcher and only owns the CAS-specific actor/org policy: unlike
// the control plane's biz.AuditorUseCase, the actor is always SYSTEM (CAS JWTs
// carry no user identity) and the org comes from the JWT claims instead of the
// request context.
type AuditDispatcher struct {
	dispatcher *auditor.Dispatcher
	log        *log.Helper
}

func NewAuditDispatcher(publisher *auditor.AuditLogPublisher, logger log.Logger) *AuditDispatcher {
	// keep the Publisher interface nil when the publisher is disabled so the
	// dispatcher short-circuits instead of holding a typed-nil interface
	var p auditor.Publisher
	if publisher != nil {
		p = publisher
	}

	return &AuditDispatcher{
		dispatcher: auditor.NewDispatcher(p, logger),
		log:        servicelogger.ScopedHelper(logger, "audit-dispatcher"),
	}
}

// shouldEmit returns true when Dispatch would actually publish an event for the
// given claims. Hooks use it to skip extra work (e.g. backend Describe round-trips).
func (d *AuditDispatcher) shouldEmit(claims *casJWT.Claims) bool {
	return d != nil && d.dispatcher.Enabled() && claims != nil && !claims.SourceInternal
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

	d.dispatcher.Dispatch(entry,
		auditor.WithActor(auditor.ActorTypeSystem, uuid.Nil, "", ""),
		auditor.WithOrgID(orgID),
	)
}
