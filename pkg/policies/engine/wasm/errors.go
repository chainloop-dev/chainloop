//
// Copyright 2024-2025 The Chainloop Authors.
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

package wasm

import (
	"fmt"
	"regexp"
	"strings"
)

// ErrorCategory represents the type of WASM policy error
type ErrorCategory string

const (
	CategoryHTTPForbidden ErrorCategory = "http_forbidden"
	CategoryTimeout       ErrorCategory = "timeout"
	CategoryMemory        ErrorCategory = "memory"
	CategoryPanic         ErrorCategory = "panic"
	CategoryUnknown       ErrorCategory = "unknown"
)

// PolicyError represents a parsed WASM policy error with user-friendly messaging
type PolicyError struct {
	Category    ErrorCategory
	UserMessage string
	Hint        string
	OriginalErr error
}

// Error patterns for common WASM execution errors
var errorPatterns = []struct {
	pattern  *regexp.Regexp
	category ErrorCategory
	extract  func(matches []string) (message, hint string)
}{
	{
		// HTTP request to disallowed hostname
		pattern:  regexp.MustCompile(`HTTP request to '(.+?)' is not allowed`),
		category: CategoryHTTPForbidden,
		extract: func(matches []string) (string, string) {
			url := matches[1]
			hostname := extractHostname(url)
			message := fmt.Sprintf("HTTP request blocked - hostname '%s' is not in the allowed hosts list", hostname)
			hint := fmt.Sprintf("Add the hostname using --allowed-hostnames flag or configure it in your policy engine.\nAttempted URL: %s", url)
			return message, hint
		},
	},
	{
		// Alternative HTTP forbidden pattern
		pattern:  regexp.MustCompile(`Host not allowed`),
		category: CategoryHTTPForbidden,
		extract: func(_ []string) (string, string) {
			message := "HTTP request blocked - hostname is not in the allowed hosts list"
			hint := "Add the hostname using --allowed-hostnames flag or configure it in your policy engine."
			return message, hint
		},
	},
	{
		// Execution timeout
		pattern:  regexp.MustCompile(`context deadline exceeded`),
		category: CategoryTimeout,
		extract: func(_ []string) (string, string) {
			message := "Policy execution timeout exceeded"
			hint := "The policy took too long to execute. Consider optimizing network calls, reducing data processing, or increasing the timeout with WithExecutionTimeout()."
			return message, hint
		},
	},
	{
		// Out of memory
		pattern:  regexp.MustCompile(`out of memory`),
		category: CategoryMemory,
		extract: func(_ []string) (string, string) {
			message := "Policy exceeded memory limits"
			hint := "Review data structures and avoid loading large files entirely into memory. Consider streaming or processing data in chunks."
			return message, hint
		},
	},
	{
		// Runtime panic with specific error
		pattern:  regexp.MustCompile(`runtime error: ([^\(]+?)(?:\s+\(recovered by wazero\)|$)`),
		category: CategoryPanic,
		extract: func(matches []string) (string, string) {
			runtimeErr := strings.TrimSpace(matches[1])
			message := fmt.Sprintf("Runtime error in policy: %s", runtimeErr)
			hint := "The policy encountered an unexpected error. Enable debug logging with --debug to see detailed stack traces."
			return message, hint
		},
	},
}

// parseWasmError parses a WASM execution error into a user-friendly PolicyError
func parseWasmError(err error) *PolicyError {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Try pattern matching for known error types
	for _, ep := range errorPatterns {
		if matches := ep.pattern.FindStringSubmatch(errStr); matches != nil {
			message, hint := ep.extract(matches)
			return &PolicyError{
				Category:    ep.category,
				UserMessage: message,
				Hint:        hint,
				OriginalErr: err,
			}
		}
	}

	// Fallback: extract first meaningful line before stack trace
	lines := strings.Split(errStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Stop at stack trace markers
		if line == "" || strings.HasPrefix(line, "wasm stack trace:") || strings.HasPrefix(line, "\t") {
			break
		}

		// Clean up wazero artifacts
		line = strings.ReplaceAll(line, " (recovered by wazero)", "")
		line = strings.TrimSpace(line)

		if line != "" {
			return &PolicyError{
				Category:    CategoryUnknown,
				UserMessage: line,
				Hint:        "Enable debug logging with --debug for more details.",
				OriginalErr: err,
			}
		}
	}

	// Ultimate fallback
	return &PolicyError{
		Category:    CategoryUnknown,
		UserMessage: "Policy execution failed",
		Hint:        "Enable debug logging with --debug for detailed error information.",
		OriginalErr: err,
	}
}

// extractHostname extracts the hostname from a URL string
func extractHostname(urlStr string) string {
	// Remove protocol
	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")

	// Extract hostname (everything before first / or :)
	if idx := strings.IndexAny(urlStr, "/:"); idx != -1 {
		return urlStr[:idx]
	}

	return urlStr
}

// Error implements the error interface for PolicyError
func (e *PolicyError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n\nHint: %s", e.UserMessage, e.Hint)
	}
	return e.UserMessage
}
