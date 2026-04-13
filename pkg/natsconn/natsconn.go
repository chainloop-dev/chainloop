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

package natsconn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
)

// Config holds the connection parameters for NATS.
// Decoupled from protobuf config so this package can be imported externally.
type Config struct {
	URI      string
	Token    string
	Name     string
	Replicas int // JetStream KV replica count; defaults to 1
}

// ReloadableConnection wraps a NATS connection and provides reconnection
// notifications via a pub/sub fan-out to subscribers.
type ReloadableConnection struct {
	*nats.Conn
	Replicas    int // JetStream KV replica count
	mu          sync.RWMutex
	subscribers []chan struct{}
	logger      *log.Helper
}

// New creates a ReloadableConnection with automatic reconnection handling.
// Returns (nil, cleanup, nil) when cfg is nil or URI is empty (NATS is optional).
// The cleanup function drains the NATS connection on shutdown.
func New(cfg *Config, logger log.Logger) (*ReloadableConnection, func(), error) {
	noop := func() {}
	if cfg == nil || cfg.URI == "" {
		return nil, noop, nil
	}

	l := log.NewHelper(log.With(logger, "component", "natsconn"))
	rc := &ReloadableConnection{logger: l, Replicas: cfg.Replicas}

	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			l.Warnw("msg", "NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			l.Infow("msg", "NATS reconnected", "url", nc.ConnectedUrl())
			rc.Broadcast()
		}),
	}

	if cfg.Name != "" {
		opts = append(opts, nats.Name(cfg.Name))
	}

	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}

	nc, err := nats.Connect(cfg.URI, opts...)
	if err != nil {
		return nil, noop, fmt.Errorf("connecting to NATS: %w", err)
	}

	rc.Conn = nc
	l.Infow("msg", "NATS connected", "url", nc.ConnectedUrl())

	cleanup := func() {
		l.Infow("msg", "draining NATS connection")
		if err := nc.Drain(); err != nil {
			l.Warnw("msg", "failed to drain NATS connection", "error", err)
		}
	}

	return rc, cleanup, nil
}

// Subscribe registers for reconnection notifications. The returned channel
// receives a signal each time the NATS connection is re-established.
// The subscription is automatically removed when ctx is cancelled.
// Nil-receiver safe: returns a closed channel.
func (rc *ReloadableConnection) Subscribe(ctx context.Context) <-chan struct{} {
	if rc == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	ch := make(chan struct{}, 1)

	rc.mu.Lock()
	rc.subscribers = append(rc.subscribers, ch)
	rc.mu.Unlock()

	go func() {
		<-ctx.Done()
		rc.unsubscribe(ch)
	}()

	return ch
}

func (rc *ReloadableConnection) unsubscribe(ch chan struct{}) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for i, s := range rc.subscribers {
		if s == ch {
			rc.subscribers = append(rc.subscribers[:i], rc.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Broadcast notifies all subscribers of a reconnection event.
// Non-blocking: if a subscriber's channel is full, the signal is dropped.
// Nil-receiver safe.
func (rc *ReloadableConnection) Broadcast() {
	if rc == nil {
		return
	}

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	for _, ch := range rc.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
