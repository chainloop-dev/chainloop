// Copyright 2025-2026 The Chainloop Authors.
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

package policydevel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr)
	policyPath := "testdata/policy-test.yaml"

	t.Run("evaluation with explicit kind", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0600))

		opts := &EvalOptions{
			PolicyPath:   policyPath,
			MaterialKind: "STRING",
			MaterialPath: testFile,
			Annotations:  map[string]string{"key": "value"},
		}

		results, err := Evaluate(opts, logger)
		require.Error(t, err)
		assert.Empty(t, results)
	})

	t.Run("evaluation with auto-detected SBOM CYCLONEDX kind", func(t *testing.T) {
		materialPath := "testdata/sbom_cyclonedx.json"

		opts := &EvalOptions{
			PolicyPath:   policyPath,
			MaterialKind: "",
			MaterialPath: materialPath,
			Annotations:  map[string]string{"key": "value"},
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)

		if len(result.Result.Violations) == 0 {
			t.Log("Policy evaluation passed (no violations)")
		} else {
			for _, violation := range result.Result.Violations {
				t.Logf("Violation: %s", violation)
			}
		}
	})

	t.Run("evaluation with auto-detected ATTESTATION kind", func(t *testing.T) {
		materialPath := "testdata/attestation.json"

		opts := &EvalOptions{
			PolicyPath:   policyPath,
			MaterialKind: "",
			MaterialPath: materialPath,
			Annotations:  map[string]string{"key": "value"},
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)

		if len(result.Result.Violations) == 0 {
			t.Log("Policy evaluation passed (no violations)")
		} else {
			for _, violation := range result.Result.Violations {
				t.Logf("Violation: %s", violation)
			}
		}
	})

	t.Run("invalid policy content", func(t *testing.T) {
		policyPath := filepath.Join(tempDir, "invalid_policy.yaml")
		require.NoError(t, os.WriteFile(policyPath, []byte("invalid policy content"), 0600))

		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0600))

		opts := &EvalOptions{
			PolicyPath:   policyPath,
			MaterialKind: "STRING",
			MaterialPath: testFile,
		}

		_, err := Evaluate(opts, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load policy spec")
	})

	t.Run("invalid material kind", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0600))

		opts := &EvalOptions{
			PolicyPath:   policyPath,
			MaterialKind: "INVALID_KIND",
			MaterialPath: testFile,
		}

		_, err := Evaluate(opts, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid material kind")
	})
}

func TestEvaluateSimplifiedPolicies(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr)

	sbomContent, err := os.ReadFile("testdata/test-sbom.json")
	require.NoError(t, err)
	sbomPath := filepath.Join(tempDir, "test-sbom.json")
	require.NoError(t, os.WriteFile(sbomPath, sbomContent, 0600))

	t.Run("sbom min components policy", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-min-components-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Result.Skipped)
		assert.Len(t, result.Result.Violations, 1)
		assert.Contains(t, string(result.Result.Violations[0]), "at least 2 components")
	})

	t.Run("violations with finding_type use unified format matching attestation storage", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-structured-vuln-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Result.Skipped)

		// Single unified violations field with full violation objects (same as attestation)
		require.Len(t, result.Result.Violations, 1)

		var v map[string]any
		require.NoError(t, json.Unmarshal(result.Result.Violations[0], &v))
		assert.Nil(t, v["subject"], "subject should be excluded from eval output")
		assert.Contains(t, v["message"], "Vulnerability found in test-component@1.0.0")

		vuln, ok := v["vulnerability"].(map[string]any)
		require.True(t, ok, "expected vulnerability finding in violation object")
		assert.Equal(t, "CVE-2024-1234", vuln["external_id"])
		assert.Equal(t, "pkg:generic/test-component@1.0.0", vuln["package_purl"])
		assert.Equal(t, "HIGH", vuln["severity"])
		assert.InDelta(t, 7.5, vuln["cvss_v3_score"], 0.001)
	})

	t.Run("violations without finding_type use same unified format", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-min-components-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Result.Violations, 1)

		// Same structure as attestation: object with message (subject excluded in eval)
		var v map[string]any
		require.NoError(t, json.Unmarshal(result.Result.Violations[0], &v))
		assert.Nil(t, v["subject"], "subject should be excluded from eval output")
		assert.Contains(t, v["message"], "at least 2 components")
	})

	t.Run("sbom metadata component policy", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-metadata-component-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Result.Skipped)
		assert.Len(t, result.Result.Violations, 0)
	})

	t.Run("sbom valid cyclonedx policy", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-valid-cyclonedx-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Result.Skipped)
		assert.Len(t, result.Result.Violations, 0)
	})

	t.Run("sbom multiple checks policy", func(t *testing.T) {
		opts := &EvalOptions{
			PolicyPath:   "testdata/sbom-multiple-checks-policy.yaml",
			MaterialPath: sbomPath,
		}

		result, err := Evaluate(opts, logger)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Result.Skipped)
		assert.Len(t, result.Result.Violations, 1)
		assert.Contains(t, string(result.Result.Violations[0]), "too few components")
	})
}
