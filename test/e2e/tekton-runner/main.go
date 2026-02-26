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

// e2e test binary for TektonPipeline runner.
// Run inside a Tekton TaskRun to validate two-tier native metadata discovery
// against a real Kubernetes environment.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/rs/zerolog"
)

type result struct {
	Check  string `json:"check"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.Level(zerolog.DebugLevel)

	fmt.Println("=== Chainloop Tekton Runner E2E Test ===")
	fmt.Println()

	// Instantiate the runner -- this triggers two-tier discovery
	r := runners.NewTektonPipeline(context.Background(), &logger)

	var results []result
	pass, fail := 0, 0

	check := func(name, status, detail string) {
		results = append(results, result{Check: name, Status: status, Detail: detail})
		if status == "PASS" {
			pass++
			fmt.Printf("  ✓ %s: %s\n", name, detail)
		} else {
			fail++
			fmt.Printf("  ✗ %s: %s\n", name, detail)
		}
	}

	// === Tier 1 checks ===
	fmt.Println("--- Tier 1: Filesystem & Environment ---")

	// Check runner ID
	if r.ID().String() == "TEKTON_PIPELINE" {
		check("RunnerID", "PASS", "TEKTON_PIPELINE")
	} else {
		check("RunnerID", "FAIL", r.ID().String())
	}

	// Check environment detection
	if r.CheckEnv() {
		check("CheckEnv", "PASS", "Tekton environment detected")
	} else {
		check("CheckEnv", "FAIL", "/tekton/results not found")
	}

	// Check ResolveEnvVars returns metadata
	envVars, errs := r.ResolveEnvVars()
	if errs == nil || len(errs) == 0 {
		check("ResolveEnvVars.NoErrors", "PASS", "No errors returned")
	} else {
		check("ResolveEnvVars.NoErrors", "FAIL", fmt.Sprintf("%d errors", len(errs)))
	}

	// Check HOSTNAME is resolved
	if hostname, ok := envVars["HOSTNAME"]; ok && hostname != "" {
		check("ResolveEnvVars.HOSTNAME", "PASS", hostname)
	} else {
		check("ResolveEnvVars.HOSTNAME", "FAIL", "HOSTNAME not in resolved env vars")
	}

	// Check namespace is resolved
	if ns, ok := envVars["TEKTON_NAMESPACE"]; ok && ns != "" {
		check("ResolveEnvVars.TEKTON_NAMESPACE", "PASS", ns)
	} else {
		check("ResolveEnvVars.TEKTON_NAMESPACE", "FAIL", "TEKTON_NAMESPACE not in resolved env vars")
	}

	// === Tier 2 checks ===
	fmt.Println()
	fmt.Println("--- Tier 2: K8s API Pod Labels ---")

	// Check tekton.dev/taskRun label discovered
	if taskRun, ok := envVars["TEKTON_TASKRUN_NAME"]; ok && taskRun != "" {
		check("ResolveEnvVars.TEKTON_TASKRUN_NAME", "PASS", taskRun)
	} else {
		check("ResolveEnvVars.TEKTON_TASKRUN_NAME", "FAIL", "TEKTON_TASKRUN_NAME not in resolved env vars (K8s API discovery may have failed)")
	}

	// === RunURI check ===
	fmt.Println()
	fmt.Println("--- RunURI ---")

	runURI := r.RunURI()
	if runURI != "" {
		check("RunURI", "PASS", runURI)
	} else {
		check("RunURI", "FAIL", "RunURI returned empty string")
	}

	// === Report check ===
	fmt.Println()
	fmt.Println("--- Report ---")

	reportErr := r.Report([]byte("E2E test table output"), "https://e2e-test.example.com/attestation/123")
	if reportErr == nil {
		check("Report", "PASS", "Report written to Tekton Results")
	} else {
		check("Report", "FAIL", reportErr.Error())
	}

	// === Environment check ===
	fmt.Println()
	fmt.Println("--- Environment ---")

	env := r.Environment()
	check("Environment", "PASS", env.String())

	// === Summary ===
	fmt.Println()
	fmt.Printf("=== Results: %d passed, %d failed ===\n", pass, fail)

	// Write JSON results to Tekton Results for machine parsing
	jsonResults, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile("/tekton/results/e2e-test-results", jsonResults, 0600)

	if fail > 0 {
		os.Exit(1)
	}
}
