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

package auditor

import (
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/log"
)

// Publisher publishes generated audit event payloads to the event bus.
// Implemented by *AuditLogPublisher; abstracted so it can be faked in tests and
// so a nil publisher can act as a no-op (NATS not configured).
type Publisher interface {
	Publish(data *EventPayload) error
}

// Dispatcher centralizes the generate -> publish -> error-reporting flow shared
// by every component that emits audit events (e.g. the control plane's
// biz.AuditorUseCase and the Artifact CAS). Callers resolve the actor and
// organization themselves and pass them as GeneratorOptions, so each component
// keeps its own actor/org policy (request context vs JWT claims) while sharing
// the common dispatch machinery.
type Dispatcher struct {
	// nil when the publisher is not configured, making the dispatcher a no-op
	publisher Publisher
	log       *log.Helper
}

// NewDispatcher builds a Dispatcher. A nil publisher (e.g. NATS not configured)
// turns Dispatch into a no-op and makes Enabled report false.
func NewDispatcher(publisher Publisher, logger log.Logger) *Dispatcher {
	return &Dispatcher{
		publisher: publisher,
		log:       servicelogger.ScopedHelper(logger, "auditor-dispatcher"),
	}
}

// Enabled reports whether Dispatch would actually publish an event. Callers can
// use it to skip extra work when the dispatcher is a no-op.
func (d *Dispatcher) Enabled() bool {
	return d != nil && d.publisher != nil
}

// Dispatch generates the audit event from entry and the given options and
// publishes it. Best-effort: failures are logged and reported to Sentry, never
// returned, so they can't fail or slow down the caller. A disabled dispatcher
// is a no-op.
func (d *Dispatcher) Dispatch(entry LogEntry, opts ...GeneratorOption) {
	if !d.Enabled() {
		return
	}

	payload, err := GenerateAuditEvent(entry, opts...)
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
