//go:build tinygo.wasm

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

package main

import (
	"encoding/json"
	"os"

	"github.com/extism/go-pdk"
)

type Result struct {
	Violations []string `json:"violations"`
	Skipped    bool     `json:"skipped"`
}

// Execute attempts to access various host filesystem paths
//
//export Execute
func Execute() int32 {
	result := Result{Violations: []string{}, Skipped: false}

	// Attempt to stat /etc/passwd (common on Unix systems)
	// Using Stat instead of ReadFile to avoid potential memory issues
	_, err1 := os.Stat("/etc/passwd")
	if err1 == nil {
		result.Violations = append(result.Violations, "SECURITY VIOLATION: Successfully accessed /etc/passwd")
	}

	// Attempt to stat /etc/hosts
	_, err2 := os.Stat("/etc/hosts")
	if err2 == nil {
		result.Violations = append(result.Violations, "SECURITY VIOLATION: Successfully accessed /etc/hosts")
	}

	// Attempt to stat current directory
	_, err3 := os.Stat(".")
	if err3 == nil {
		result.Violations = append(result.Violations, "SECURITY VIOLATION: Successfully accessed current directory")
	}

	// Attempt to stat root directory
	_, err4 := os.Stat("/")
	if err4 == nil {
		result.Violations = append(result.Violations, "SECURITY VIOLATION: Successfully accessed root directory")
	}

	// Output result
	output, err := json.Marshal(result)
	if err != nil {
		return 1
	}
	mem := pdk.AllocateBytes(output)
	pdk.OutputMemory(mem)
	return 0
}

func main() {}
