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
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records published payloads and optionally fails.
type fakePublisher struct {
	published []*EventPayload
	err       error
}

func (f *fakePublisher) Publish(data *EventPayload) error {
	if f.err != nil {
		return f.err
	}

	f.published = append(f.published, data)
	return nil
}

// testLogEntry is a minimal LogEntry implementation for dispatcher tests.
type testLogEntry struct {
	description string
}

func (e *testLogEntry) ActionType() string                   { return "TEST_ACTION" }
func (e *testLogEntry) ActionInfo() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
func (e *testLogEntry) TargetType() TargetType               { return "TEST_TARGET" }
func (e *testLogEntry) TargetID() *uuid.UUID                 { return nil }
func (e *testLogEntry) Description() string                  { return e.description }
func (e *testLogEntry) RequiresActor() bool                  { return false }

func validEntry() LogEntry {
	return &testLogEntry{description: "something happened"}
}

func systemActor() GeneratorOption {
	return WithActor(ActorTypeSystem, uuid.Nil, "", "")
}

func TestDispatcherEnabled(t *testing.T) {
	tests := []struct {
		name       string
		dispatcher *Dispatcher
		want       bool
	}{
		{name: "nil dispatcher", dispatcher: nil, want: false},
		{name: "nil publisher", dispatcher: NewDispatcher(nil, log.DefaultLogger), want: false},
		{name: "configured publisher", dispatcher: NewDispatcher(&fakePublisher{}, log.DefaultLogger), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.dispatcher.Enabled())
		})
	}
}

func TestDispatcherDispatch(t *testing.T) {
	tests := []struct {
		name          string
		publisher     *fakePublisher
		entry         LogEntry
		opts          []GeneratorOption
		wantPublished int
	}{
		{
			name:          "publishes a valid event",
			publisher:     &fakePublisher{},
			entry:         validEntry(),
			opts:          []GeneratorOption{systemActor()},
			wantPublished: 1,
		},
		{
			name:      "generation failure is swallowed",
			publisher: &fakePublisher{},
			// empty description makes GenerateAuditEvent fail
			entry: &testLogEntry{description: ""},
			opts:  []GeneratorOption{systemActor()},
		},
		{
			name:      "publish errors are swallowed",
			publisher: &fakePublisher{err: errors.New("nats is down")},
			entry:     validEntry(),
			opts:      []GeneratorOption{systemActor()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDispatcher(tc.publisher, log.DefaultLogger)
			// must never panic nor return an error
			d.Dispatch(tc.entry, tc.opts...)

			require.Len(t, tc.publisher.published, tc.wantPublished)
		})
	}
}

func TestDispatcherDispatchNoOpWhenDisabled(t *testing.T) {
	// nil publisher and nil dispatcher must both be safe no-ops
	assert.NotPanics(t, func() {
		NewDispatcher(nil, log.DefaultLogger).Dispatch(validEntry(), systemActor())

		var d *Dispatcher
		d.Dispatch(validEntry(), systemActor())
	})
}
