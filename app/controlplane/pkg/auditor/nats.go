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
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamName  = "chainloop-audit"
	subjectName = "audit.>"
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
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	return n.conn.Publish(subjectName, jsonPayload)
}
