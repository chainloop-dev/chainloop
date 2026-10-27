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

// Package errorsxtest helps tests check the origin stack that Sentry records for an error.
package errorsxtest

import "github.com/getsentry/sentry-go"

// OriginStack returns the functions of the first stack in exceptions, innermost
// first. sentry-go orders exceptions root-first, so an origin stack comes before
// the stack of the capture site. It returns nil when no exception has a stack.
func OriginStack(exceptions []sentry.Exception) []string {
	for _, exc := range exceptions {
		if exc.Stacktrace == nil || len(exc.Stacktrace.Frames) == 0 {
			continue
		}

		// Sentry frames are outermost first.
		frames := exc.Stacktrace.Frames
		fns := make([]string, 0, len(frames))
		for i := len(frames) - 1; i >= 0; i-- {
			fns = append(fns, frames[i].Function)
		}

		return fns
	}

	return nil
}

// OriginFunction returns the innermost function of OriginStack, or "".
func OriginFunction(exceptions []sentry.Exception) string {
	if stack := OriginStack(exceptions); len(stack) > 0 {
		return stack[0]
	}

	return ""
}

// ErrorExceptions returns the exceptions that Sentry records for err.
func ErrorExceptions(err error) []sentry.Exception {
	event := sentry.NewEvent()
	event.SetException(err, 10)

	return event.Exception
}

// ErrorOriginFunction returns OriginFunction of the exceptions that Sentry records for err.
func ErrorOriginFunction(err error) string {
	return OriginFunction(ErrorExceptions(err))
}
