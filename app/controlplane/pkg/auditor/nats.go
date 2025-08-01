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

package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
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
	conn   *nats.Conn
	logger *log.Helper
}

func NewAuditLogPublisher(conn *nats.Conn, logger log.Logger) (*AuditLogPublisher, error) {
	l := log.NewHelper(log.With(logger, "component", "natsAuditLogPublisher"))
	if conn == nil {
		l.Infow("msg", "NATS connection not set, audit log publisher disabled")
		return nil, nil
	}

	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{subjectName},
	}); err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	l.Infow("msg", "Stream Created or Updated", "name", streamName, "subject", subjectName)

	return &AuditLogPublisher{conn, l}, nil
}

func (n *AuditLogPublisher) Publish(data *EventPayload) error {
	// If the connection is nil, we don't want to publish anything
	if n == nil || n.conn == nil {
		return nil
	}

	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Send the event to the specific subject based on the event type "audit.<target_type>.<action_type>"
	specificSubject := fmt.Sprintf("%s.%s.%s", baseSubjectName, strings.ToLower(string(data.Data.TargetType)), strings.ToLower(data.Data.ActionType))
	return n.conn.Publish(specificSubject, jsonPayload)
}
