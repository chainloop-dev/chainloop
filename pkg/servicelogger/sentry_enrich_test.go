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

package servicelogger_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/getsentry/sentry-go"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeStateError mimics pgconn.PgError's SQLState() string interface
type fakeStateError struct{ code string }

func (e *fakeStateError) Error() string    { return "duplicate key value violates unique constraint" }
func (e *fakeStateError) SQLState() string { return e.code }

// cycleError builds cyclic unwrap chains
type cycleError struct {
	msg  string
	next *cycleError
}

func (e *cycleError) Error() string { return e.msg }
func (e *cycleError) Unwrap() error {
	if e.next == nil {
		return nil
	}
	return e.next
}

// sqlAndGRPCError implements both SQLState and GRPCStatus to pin discriminator precedence
type sqlAndGRPCError struct{}

func (e *sqlAndGRPCError) Error() string    { return "db unavailable" }
func (e *sqlAndGRPCError) SQLState() string { return "08006" }
func (e *sqlAndGRPCError) GRPCStatus() *status.Status {
	return status.New(codes.Unavailable, "db unavailable")
}

// nilStatusError implements GRPCStatus but returns nil
type nilStatusError struct{}

func (e *nilStatusError) Error() string              { return "nil status" }
func (e *nilStatusError) GRPCStatus() *status.Status { return nil }

func newErrorEvent(exceptions ...sentry.Exception) *sentry.Event {
	return &sentry.Event{Exception: exceptions}
}

// primary returns the last (outermost) exception, the one Sentry uses for the issue title
func primary(event *sentry.Event) sentry.Exception {
	return event.Exception[len(event.Exception)-1]
}

func TestSentryBeforeSend(t *testing.T) {
	t.Run("wrapped SQLSTATE error gets discriminator fingerprint", func(t *testing.T) {
		root := &fakeStateError{code: "23505"}
		err := fmt.Errorf("creating version: %w", root)
		event := newErrorEvent(
			sentry.Exception{Type: "*servicelogger_test.fakeStateError", Value: root.Error()},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})
		require.NotNil(t, got)

		assert.Equal(t, []string{"{{ default }}", "23505"}, got.Fingerprint)
		// generic wrapper type rewritten to the root-cause type
		assert.Equal(t, "*servicelogger_test.fakeStateError", primary(got).Type)
		assert.Equal(t, map[string]string{
			"error.root_type":   "*servicelogger_test.fakeStateError",
			"error.chain_depth": "1",
			"error.sqlstate":    "23505",
		}, got.Tags)
	})

	t.Run("nested wraps report correct chain depth", func(t *testing.T) {
		err := fmt.Errorf("storing attestation: %w", fmt.Errorf("querying db: %w", errors.New("connection reset")))
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "connection reset"},
			sentry.Exception{Type: "*fmt.wrapError", Value: "querying db: connection reset"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		// no structured discriminator → default grouping; chain depth still tagged
		assert.Empty(t, got.Fingerprint)
		assert.Equal(t, "2", got.Tags["error.chain_depth"])
	})

	t.Run("wrapped gRPC status error discriminates by code", func(t *testing.T) {
		err := fmt.Errorf("creating the gRPC client: %w", status.Error(codes.DeadlineExceeded, "context deadline exceeded"))
		event := newErrorEvent(
			sentry.Exception{Type: "*status.Error", Value: "rpc error: code = DeadlineExceeded desc = context deadline exceeded"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Equal(t, []string{"{{ default }}", "DeadlineExceeded"}, got.Fingerprint)
		assert.Equal(t, "DeadlineExceeded", got.Tags["error.grpc_code"])
	})

	t.Run("wrapped kratos error tags reason and gRPC code", func(t *testing.T) {
		kratosErr := kerrors.New(500, "db unavailable", "database is closed")
		err := fmt.Errorf("loading integration info: %w", kratosErr)
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.Error", Value: kratosErr.Error()},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Equal(t, []string{"{{ default }}", "Internal"}, got.Fingerprint)
		assert.Equal(t, "Internal", got.Tags["error.grpc_code"])
		assert.Equal(t, "db unavailable", got.Tags["error.kratos_reason"])
	})

	t.Run("bare kratos error groups by reason", func(t *testing.T) {
		err := kerrors.New(500, "internal error", "server error")
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.Error", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Equal(t, []string{"{{ default }}", "Internal"}, got.Fingerprint)
		// non-generic type is not rewritten
		assert.Equal(t, "*errors.Error", primary(got).Type)
	})

	t.Run("plain unwrapped error keeps default grouping", func(t *testing.T) {
		err := errors.New("boom")
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "boom"},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Empty(t, got.Fingerprint)
		// tags are still attached for searchability
		assert.Equal(t, "*errors.errorString", got.Tags["error.root_type"])
		assert.Equal(t, "0", got.Tags["error.chain_depth"])
	})

	t.Run("error without structured discriminator keeps default grouping", func(t *testing.T) {
		err := fmt.Errorf("doing work: %w", errors.New("root cause"))
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "root cause"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Empty(t, got.Fingerprint)
	})

	t.Run("nil GRPCStatus implementation does not panic", func(t *testing.T) {
		err := fmt.Errorf("calling upstream: %w", &nilStatusError{})
		event := newErrorEvent(
			sentry.Exception{Type: "*servicelogger_test.nilStatusError", Value: "nil status"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		// nil status → no grpc code → no discriminator → default grouping
		assert.Empty(t, got.Fingerprint)
		_, hasGRPCCode := got.Tags["error.grpc_code"]
		assert.False(t, hasGRPCCode)
	})

	t.Run("cyclic chain terminates", func(t *testing.T) {
		a := &cycleError{msg: "a"}
		b := &cycleError{msg: "b"}
		a.next, b.next = b, a
		event := newErrorEvent(
			sentry.Exception{Type: "*servicelogger_test.cycleError", Value: "a"},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: a})

		// the observable contract is termination with a bounded depth
		depth, err := strconv.Atoi(got.Tags["error.chain_depth"])
		require.NoError(t, err)
		// 32 mirrors the unexported maxUnwrapDepth constant in the production
		// package (external test package cannot reference it)
		assert.LessOrEqual(t, depth, 32)
	})

	t.Run("recovered panic error is analyzed", func(t *testing.T) {
		err := fmt.Errorf("booting server: %w", errors.New("out of fds"))
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "out of fds"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{RecoveredException: err})

		// no structured discriminator → default grouping, but tags are set
		assert.Empty(t, got.Fingerprint)
		assert.Equal(t, "*errors.errorString", got.Tags["error.root_type"])
	})

	t.Run("existing tags are preserved", func(t *testing.T) {
		err := fmt.Errorf("creating version: %w", errors.New("conflict"))
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "conflict"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)
		event.Tags = map[string]string{"request_id": "abc"}

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Equal(t, "abc", got.Tags["request_id"])
		assert.Equal(t, "*errors.errorString", got.Tags["error.root_type"])
	})

	t.Run("joined multi-errors keep default grouping but are tagged", func(t *testing.T) {
		err := fmt.Errorf("reconciling: %w", errors.Join(
			errors.New("first failure"),
			&fakeStateError{code: "23505"},
		))
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "first failure"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		// joined failures are ambiguous: no single root defines them, so no
		// fingerprint or type rewrite; metadata from all branches is tagged
		assert.Empty(t, got.Fingerprint)
		assert.Equal(t, "*fmt.wrapError", primary(got).Type)
		assert.Equal(t, "23505", got.Tags["error.sqlstate"])
		assert.Equal(t, "true", got.Tags["error.multi_error"])
		_, hasRootType := got.Tags["error.root_type"]
		assert.False(t, hasRootType)
	})

	t.Run("unwrapped SQLSTATE error gets discriminator fingerprint", func(t *testing.T) {
		err := &fakeStateError{code: "23505"}
		event := newErrorEvent(
			sentry.Exception{Type: "*servicelogger_test.fakeStateError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		// SQLSTATE is a structured discriminator → fingerprint extends default grouping
		assert.Equal(t, []string{"{{ default }}", "23505"}, got.Fingerprint)
		assert.Equal(t, "23505", got.Tags["error.sqlstate"])
	})

	t.Run("same root type with different dynamic values groups together", func(t *testing.T) {
		root := &fakeStateError{code: "23505"}
		makeEvent := func(email string) *sentry.Event {
			err := fmt.Errorf("error finding user %s: %w", email, root)
			return newErrorEvent(
				sentry.Exception{Type: "*servicelogger_test.fakeStateError", Value: root.Error()},
				sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
			)
		}

		first := servicelogger.SentryBeforeSend(makeEvent("alice@corp.com"), &sentry.EventHint{OriginalException: fmt.Errorf("error finding user %s: %w", "alice@corp.com", root)})
		second := servicelogger.SentryBeforeSend(makeEvent("bob@other.io"), &sentry.EventHint{OriginalException: fmt.Errorf("error finding user %s: %w", "bob@other.io", root)})

		// the fingerprint uses only the structured discriminator, not message
		// text, so different request-specific values produce the same fingerprint
		assert.Equal(t, first.Fingerprint, second.Fingerprint)
	})

	t.Run("SQLSTATE takes precedence over gRPC code as discriminator", func(t *testing.T) {
		err := fmt.Errorf("reconnecting: %w", &sqlAndGRPCError{})
		event := newErrorEvent(
			sentry.Exception{Type: "*servicelogger_test.sqlAndGRPCError", Value: "db unavailable"},
			sentry.Exception{Type: "*fmt.wrapError", Value: err.Error()},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: err})

		assert.Equal(t, []string{"{{ default }}", "08006"}, got.Fingerprint)
		assert.Equal(t, "08006", got.Tags["error.sqlstate"])
		assert.Equal(t, "Unavailable", got.Tags["error.grpc_code"])
	})

	t.Run("non-error recovered panic is left untouched", func(t *testing.T) {
		event := newErrorEvent(
			sentry.Exception{Type: "*errors.errorString", Value: "boom"},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{RecoveredException: "boom"})

		assert.Empty(t, got.Fingerprint)
		assert.Empty(t, got.Tags)
	})

	t.Run("event without original error is left untouched", func(t *testing.T) {
		event := newErrorEvent(
			sentry.Exception{Type: "*fmt.wrapError", Value: "creating version: boom"},
		)

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{})

		assert.Empty(t, got.Fingerprint)
		assert.Empty(t, got.Tags)
		assert.Equal(t, "*fmt.wrapError", primary(got).Type)
	})

	t.Run("event without exceptions is left untouched", func(t *testing.T) {
		event := &sentry.Event{Message: "a log message"}

		got := servicelogger.SentryBeforeSend(event, &sentry.EventHint{OriginalException: errors.New("boom")})

		assert.Equal(t, "a log message", got.Message)
		assert.Empty(t, got.Fingerprint)
	})

	t.Run("nil event does not panic", func(t *testing.T) {
		assert.Nil(t, servicelogger.SentryBeforeSend(nil, nil))
	})
}
