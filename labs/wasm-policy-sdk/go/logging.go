//
// Copyright 2025 The Chainloop Authors.
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

package _go

import (
	"fmt"

	"github.com/extism/go-pdk"
)

// LogInfo logs an informational message.
// These logs are visible in the Chainloop CLI when running with --debug.
func LogInfo(format string, args ...interface{}) {
	pdk.Log(pdk.LogInfo, fmt.Sprintf(format, args...))
}

// LogDebug logs a debug message.
// These logs are visible in the Chainloop CLI when running with --debug.
func LogDebug(format string, args ...interface{}) {
	pdk.Log(pdk.LogDebug, fmt.Sprintf(format, args...))
}

// LogWarn logs a warning message.
func LogWarn(format string, args ...interface{}) {
	pdk.Log(pdk.LogWarn, fmt.Sprintf(format, args...))
}

// LogError logs an error message.
func LogError(format string, args ...interface{}) {
	pdk.Log(pdk.LogError, fmt.Sprintf(format, args...))
}

// LogTrace logs a trace message for detailed debugging.
func LogTrace(format string, args ...interface{}) {
	pdk.Log(pdk.LogTrace, fmt.Sprintf(format, args...))
}
