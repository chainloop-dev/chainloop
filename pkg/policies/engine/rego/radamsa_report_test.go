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

package rego

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minIterationsGate is a representative min-iterations gate — the count(input.elements)
// shape a real fuzzing-coverage policy uses — authored here as test scaffolding. It
// is deliberately not a copy of any shipped policy (those live in their own repo);
// what matters for this test is only that the gate reads input.elements as an array
// and counts it.
const minIterationsGate = `package radamsa_report_gate_test

import rego.v1

result := {"skipped": skipped, "violations": violations, "skip_reason": skip_reason}

default skip_reason := ""
skip_reason := "input.elements is not an array" if not valid_input

default skipped := true
skipped := false if valid_input

valid_input if is_array(input.elements)

default min_iterations := 100000
min_iterations := to_number(input.args.min_iterations) if input.args.min_iterations

iteration_count := count(input.elements)

violations contains msg if {
	valid_input
	iteration_count < min_iterations
	msg := sprintf("records %d fuzzing iteration(s), fewer than the required minimum of %d", [iteration_count, min_iterations])
}
`

// TestRadamsaMinIterationsAgainstArchiveMaterial is the end-to-end guard the
// synthetic-elements policy tests cannot provide: it runs a min-iterations gate
// over the input the ingestion pipeline actually produces for a RADAMSA_REPORT
// material whose value is an archive of per-run -M logs. It proves the gate
// ENFORCES (skipped == false) on the merged record count, rather than skipping
// because input.elements never got populated — the false-assurance failure mode
// when an archive is stored whole and not merged.
func TestRadamsaMinIterationsAgainstArchiveMaterial(t *testing.T) {
	policy := &engine.Policy{Name: "radamsa-min-iterations", Source: []byte(minIterationsGate)}

	// Each log holds three -M records.
	logA := []byte("seed: 1\nmuta-num: 1, generator: file\nbyte-dec: 1, generator: jump\n")
	logB := []byte("seed: 2\nmuta-num: 2, generator: file\nbyte-dec: 2, generator: jump\n")
	badLog := []byte("this is not a radamsa metadata log")

	dir := t.TempDir()
	// tar.gz and zip of two good logs => six merged records.
	sixTar := filepath.Join(dir, "six.tar.gz")
	writeTarGzFile(t, sixTar, map[string][]byte{"meta_1.log": logA, "meta_2.log": logB})
	sixZip := filepath.Join(dir, "six.zip")
	writeZipFile(t, sixZip, map[string][]byte{"meta_1.log": logA, "meta_2.log": logB})
	// zip with one malformed entry => the whole material is rejected at ingestion.
	withBad := filepath.Join(dir, "withbad.zip")
	writeZipFile(t, withBad, map[string][]byte{"meta_1.log": logA, "broken.log": badLog})
	// plain-gzipped single log (not a tar.gz) => three records.
	gzLog := filepath.Join(dir, "single.log.gz")
	writeGzipFile(t, gzLog, logA)
	// empty archive => zero iterations => the gate must FAIL (not skip, not error).
	emptyArchive := filepath.Join(dir, "empty.tar.gz")
	writeTarGzFile(t, emptyArchive, nil)

	tests := []struct {
		name          string
		path          string
		minIterations string
		wantViolation bool
		wantIngestErr bool // malformed content is rejected before evaluation
	}{
		{name: "tar.gz meets minimum", path: sixTar, minIterations: "6", wantViolation: false},
		{name: "tar.gz misses minimum", path: sixTar, minIterations: "7", wantViolation: true},
		{name: "zip meets minimum", path: sixZip, minIterations: "6", wantViolation: false},
		{name: "zip misses minimum", path: sixZip, minIterations: "7", wantViolation: true},
		{name: "plain gzip single log meets minimum", path: gzLog, minIterations: "3", wantViolation: false},
		{name: "plain gzip single log misses minimum", path: gzLog, minIterations: "4", wantViolation: true},
		{name: "zero-iteration report fails the gate rather than skipping", path: emptyArchive, minIterations: "1", wantViolation: true},
		{name: "malformed archive entry is rejected at ingestion", path: withBad, minIterations: "1", wantIngestErr: true},
	}

	r := NewEngine()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &v1.Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_REPORT,
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{Name: "fuzz-report", Digest: "sha256:deadbeef"},
				},
			}
			input, err := m.GetEvaluableContent(tc.path)
			if tc.wantIngestErr {
				require.Error(t, err, "malformed evidence must be rejected, not silently dropped")
				return
			}
			require.NoError(t, err)

			result, err := r.Verify(context.TODO(), policy, input, map[string]any{"min_iterations": tc.minIterations})
			require.NoError(t, err)

			assert.False(t, result.Skipped, "gate must enforce on an archive-sourced material, not skip")
			if tc.wantViolation {
				assert.NotEmpty(t, result.Violations, "record count below the minimum must be a violation")
			} else {
				assert.Empty(t, result.Violations)
			}
		})
	}
}

func writeGzipFile(t *testing.T, path string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	gz := gzip.NewWriter(f)
	_, err = gz.Write(content)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
}

func writeTarGzFile(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
}

func writeZipFile(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}
