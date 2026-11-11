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

package errorsx

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/errorsx/errorsxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type customErr struct{ code int }

func (e *customErr) Error() string { return fmt.Sprintf("custom %d", e.code) }

func newAtOrigin() error { return New("boom") }

func errorfAtOrigin(err error) error { return Errorf("doing x: %w", err) }

func wrapAtBoundary(err error) error { return WithStack(err) }

func TestNewRecordsOriginStack(t *testing.T) {
	err := newAtOrigin()

	assert.EqualError(t, err, "boom")
	assert.Equal(t, "newAtOrigin", errorsxtest.ErrorOriginFunction(err))
}

func TestErrorfRecordsOriginStackAndWraps(t *testing.T) {
	err := errorfAtOrigin(io.EOF)

	assert.EqualError(t, err, "doing x: EOF")
	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "errorfAtOrigin", errorsxtest.ErrorOriginFunction(err))
}

func TestErrorfWithoutWrappedError(t *testing.T) {
	err := Errorf("value %d", 3)

	assert.EqualError(t, err, "value 3")
	assert.Equal(t, "TestErrorfWithoutWrappedError", errorsxtest.ErrorOriginFunction(err))
}

func TestErrorfKeepsInnerStack(t *testing.T) {
	inner := newAtOrigin()
	err := Errorf("outer: %w", inner)

	assert.EqualError(t, err, "outer: boom")
	require.ErrorIs(t, err, inner)
	assert.IsType(t, fmt.Errorf("x: %w", io.EOF), err, "Errorf must not record a second stack")
	assert.Equal(t, "newAtOrigin", errorsxtest.ErrorOriginFunction(err))
}

func TestWithStack(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		assert.NoError(t, WithStack(nil))
	})

	t.Run("records the stack of the boundary", func(t *testing.T) {
		err := wrapAtBoundary(io.ErrUnexpectedEOF)

		assert.EqualError(t, err, io.ErrUnexpectedEOF.Error())
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.Equal(t, "wrapAtBoundary", errorsxtest.ErrorOriginFunction(err))
	})

	t.Run("keeps errors.As working", func(t *testing.T) {
		err := wrapAtBoundary(&customErr{code: 7})

		var target *customErr
		require.ErrorAs(t, err, &target)
		assert.Equal(t, 7, target.code)
	})

	t.Run("does not record a second stack", func(t *testing.T) {
		inner := newAtOrigin()
		wrapped := fmt.Errorf("mid: %w", inner)

		assert.Same(t, wrapped, WithStack(wrapped))
	})
}

func TestHasStack(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "plain error", err: errors.New("plain"), want: false},
		{name: "nil", err: nil, want: false},
		{name: "origin error", err: newAtOrigin(), want: true},
		{name: "wrapped origin error", err: fmt.Errorf("wrapped: %w", newAtOrigin()), want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HasStack(tc.err))
		})
	}
}
