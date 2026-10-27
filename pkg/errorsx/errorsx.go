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

// Package errorsx creates errors that record the call stack where they start.
//
// Go errors carry no stack, so sentry-go gives an event the stack of the
// CaptureException call. Every failure reported from one place then shares one
// stack, and Sentry groups unrelated causes into one issue. An error from this
// package exposes its origin stack through StackTrace() []uintptr, which
// sentry-go reads, so Sentry groups the event by where the failure started.
//
// Use New or Errorf where an error starts, and WithStack where an error from
// an external library enters the backend. Keep package-level sentinel errors
// as errors.New: a stack recorded at program start has no value.
//
// github.com/pkg/errors is not used: it is archived, its Errorf does not
// support %w, and its Wrap records a new stack on each wrap.
package errorsx

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
)

const maxStackDepth = 32

// stackTracer is the method that sentry-go reads to get the stack of an error.
type stackTracer interface {
	StackTrace() []uintptr
}

// withStack is an error with the call stack of the place that created it.
type withStack struct {
	// err is the error that this value adds a stack to.
	err error
	// stack holds the program counters of the call stack, innermost first.
	stack []uintptr
}

func (e *withStack) Error() string { return e.err.Error() }

func (e *withStack) Unwrap() error { return e.err }

// StackTrace returns the program counters of the origin stack, innermost first.
func (e *withStack) StackTrace() []uintptr { return e.stack }

// New returns an error with the message msg and the stack of the caller.
func New(msg string) error {
	return wrap(errors.New(msg))
}

// Errorf formats like fmt.Errorf, %w included. It records the stack of the
// caller, unless a wrapped error already has a stack: the origin stack is the
// one that tells failures apart.
func Errorf(format string, args ...any) error {
	return wrap(fmt.Errorf(format, args...))
}

// WithStack adds the stack of the caller to err. It returns err unchanged when
// err is nil or already has a stack.
func WithStack(err error) error {
	return wrap(err)
}

// HasStack reports whether err, or an error that it wraps, records a stack.
func HasStack(err error) bool {
	var st stackTracer

	return errors.As(err, &st)
}

// wrap must be called directly from an exported function of this package:
// the stack it records starts at the caller of that function.
func wrap(err error) error {
	if err == nil || HasStack(err) {
		return err
	}

	var pcs [maxStackDepth]uintptr
	// Skip runtime.Callers, wrap and the exported function of this package.
	n := runtime.Callers(3, pcs[:])

	return &withStack{err: err, stack: slices.Clone(pcs[:n])}
}
