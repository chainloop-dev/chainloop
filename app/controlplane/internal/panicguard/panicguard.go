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

// Package panicguard runs background work in a goroutine with a recovery frame.
//
// The gRPC recovery middleware only wraps the request handler's own stack. A
// goroutine spawned from a handler escapes it, so an unrecovered panic there
// terminates the whole process. Any request-spawned goroutine must therefore
// run through this package instead of a bare `go`.
package panicguard

import (
	"runtime/debug"

	"github.com/go-kratos/kratos/v2/log"
)

// Go runs fn in a new goroutine with a recovery frame. A recovered panic is
// logged with its stack and swallowed so it cannot crash the process.
func Go(logger *log.Helper, task string, fn func()) {
	go Recover(logger, task, fn)
}

// Recover runs fn in the current goroutine with a recovery frame. It is the
// synchronous core of Go, exposed for callers that already own a goroutine and
// for tests.
func Recover(logger *log.Helper, task string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Errorw(
					"msg", "recovered from panic in background task",
					"task", task,
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}
	}()

	fn()
}
