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

package policydevel

import (
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
