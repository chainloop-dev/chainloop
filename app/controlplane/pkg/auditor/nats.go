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

package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/natsconn"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamName = "chainloop-audit"
	// subjectName is the base subject for the stream to listen to.
	subjectName = "audit.>"
	// baseSubjectName is the base subject for audit logs for the publisher to publish to.
	// The pattern for the specific subjects is "audit.<target_type>.<action_type>"
	baseSubjectName = "audit"
)

type AuditLogPublisher struct {
	rc     *natsconn.ReloadableConnection
	logger *log.Helper
}

type PublisherOption func(*publisherOptions)

type publisherOptions struct {
	withoutStreamManagement bool
}

// WithoutStreamManagement makes the publisher publish-only: it never creates or
// updates the JetStream stream. Meant for components (e.g. the Artifact CAS) that
// publish to the stream owned and configured by the control plane, so they can't
// accidentally override its configuration (e.g. downgrade the replica count).
func WithoutStreamManagement() PublisherOption {
	return func(o *publisherOptions) {
		o.withoutStreamManagement = true
	}
}

func NewAuditLogPublisher(ctx context.Context, rc *natsconn.ReloadableConnection, logger log.Logger, opts ...PublisherOption) (*AuditLogPublisher, error) {
	l := log.NewHelper(log.With(logger, "component", "natsAuditLogPublisher"))
	if rc == nil {
		l.Infow("msg", "NATS connection not set, audit log publisher disabled")
		return nil, nil
	}

	options := &publisherOptions{}
	for _, opt := range opts {
		opt(options)
	}

	p := &AuditLogPublisher{rc: rc, logger: l}

	if options.withoutStreamManagement {
		l.Infow("msg", "stream management disabled, running in publish-only mode")
		return p, nil
	}

	if err := p.initJetStream(); err != nil {
		return nil, err
	}

	go p.watchReconnect(rc.Subscribe(ctx))

	return p, nil
}

func (p *AuditLogPublisher) initJetStream() error {
	js, err := jetstream.New(p.rc.Conn)
	if err != nil {
		return fmt.Errorf("creating jetstream context: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{subjectName},
		Replicas: p.rc.Replicas,
	}); err != nil {
		return fmt.Errorf("creating stream: %w", err)
	}

	p.logger.Infow("msg", "stream created or updated", "name", streamName, "subject", subjectName, "replicas", p.rc.Replicas)

	return nil
}

func (p *AuditLogPublisher) watchReconnect(ch <-chan struct{}) {
	for range ch {
		p.logger.Infow("msg", "NATS reconnected, reinitializing JetStream")
		if err := p.initJetStream(); err != nil {
			p.logger.Errorw("msg", "failed to reinitialize JetStream after reconnect", "error", err)
		}
	}
}

func (p *AuditLogPublisher) Publish(data *EventPayload) error {
	if p == nil || p.rc == nil {
		return nil
	}

	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling event payload: %w", err)
	}

	specificSubject := fmt.Sprintf("%s.%s.%s", baseSubjectName, strings.ToLower(string(data.Data.TargetType)), strings.ToLower(data.Data.ActionType))
	return p.rc.Publish(specificSubject, jsonPayload)
}
