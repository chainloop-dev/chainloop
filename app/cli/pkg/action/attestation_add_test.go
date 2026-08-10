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

package action

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/statemanager/filesystem"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// materialNameRe mirrors the DNS-1123-style constraint enforced on material
// names by the proto validation (name.dns-1123).
var materialNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// TestAddSourceArchiveEvidence exercises the Part B cross-link end to end: an
// exploded archive is recorded once as an EVIDENCE material and linked with the
// exploded materials in both directions.
// TestExplodeRecordsSourceArchiveEvidence checks that AddMaterialsFromArchive
// records the source archive once as an EVIDENCE material cross-linked with the
// exploded materials in both directions, all in the one atomic add.
func TestExplodeRecordsSourceArchiveEvidence(t *testing.T) {
	ctx := context.Background()

	// A dry-run crafter backed by a local state file (no control plane).
	statePath := filepath.Join(t.TempDir(), "attestation.json")
	sm, err := filesystem.New(statePath)
	require.NoError(t, err)
	c, err := crafter.NewCrafter(sm, nil)
	require.NoError(t, err)
	require.NoError(t, c.Init(ctx, &crafter.InitOpts{
		SchemaV1:      &schemaapi.CraftingSchema{SchemaVersion: "v1"},
		WfInfo:        &api.WorkflowMetadata{},
		DryRun:        true,
		AttestationID: "",
		Runner:        runners.NewGeneric(),
	}))

	// A zip of two files exploded into "scan" / "scan-1".
	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	writeZipWithFiles(t, zipPath, map[string]string{"a.txt": "a", "b.txt": "b"})

	backend := &casclient.CASBackend{}
	mts, err := c.AddMaterialsFromArchive(ctx, "", "ARTIFACT", "scan", zipPath, materials.ArchiveZip, backend, nil, materials.DefaultArchiveLimits(), crafter.WithSourceArchiveEvidence())
	require.NoError(t, err)
	require.Len(t, mts, 2)

	state := c.CraftingState.GetAttestation().GetMaterials()

	// The archive is recorded once as EVIDENCE under "scan-archive".
	ev, ok := state["scan-archive"]
	require.True(t, ok, "expected scan-archive evidence material")
	assert.Equal(t, schemaapi.CraftingSchema_Material_EVIDENCE, ev.GetMaterialType())

	// Forward edge: the archive references exactly the exploded materials.
	fwd := ev.GetAnnotations()[materials.AnnotationMaterialReferences]
	assert.ElementsMatch(t, []string{"scan", "scan-1"}, strings.Split(fwd, ","))

	// Reverse edge: every exploded material references the archive.
	for _, name := range []string{"scan", "scan-1"} {
		assert.Contains(t, state[name].GetAnnotations()[materials.AnnotationMaterialReferences], "scan-archive",
			"exploded material %q must reference the archive", name)
	}
}

func writeZipWithFiles(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}

func TestPolicyInputEvidenceNames(t *testing.T) {
	testCases := []struct {
		name         string
		materialName string
		inputs       []string
		want         []string
	}{
		{
			name:         "underscores in the input become hyphens",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths"},
			want:         []string{"sigcheck-ignored-paths"},
		},
		{
			name:         "distinct inputs keep their (sanitized) names",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths", "third_party_paths"},
			want:         []string{"sigcheck-ignored-paths", "sigcheck-third-party-paths"},
		},
		{
			name:         "same input fed by multiple files is disambiguated",
			materialName: "sigcheck",
			inputs:       []string{"ignored_paths", "ignored_paths"},
			want:         []string{"sigcheck-ignored-paths-1", "sigcheck-ignored-paths-2"},
		},
		{
			name:         "uppercase and odd characters are normalized",
			materialName: "binaries",
			inputs:       []string{"Ignored Paths!!"},
			want:         []string{"binaries-ignored-paths"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			files := make([]*PolicyInputFromFile, len(tc.inputs))
			for i, in := range tc.inputs {
				files[i] = &PolicyInputFromFile{Input: in}
			}
			got := policyInputEvidenceNames(tc.materialName, files)
			assert.Equal(t, tc.want, got)
			for _, n := range got {
				assert.Regexp(t, materialNameRe, n, "generated material name must be a valid DNS-1123 name")
			}
		})
	}
}

func TestSanitizeMaterialNamePartIsValid(t *testing.T) {
	// Even adversarial inputs must yield a part that, joined onto a material
	// name, stays a valid DNS-1123 name.
	for _, in := range []string{"ignored_paths", "  spaced  ", "UPPER", "a..b__c", "!!!", "", "_leading", "trailing_", "mixed/sep\\chars"} {
		part := sanitizeMaterialNamePart(in)
		assert.Regexp(t, materialNameRe, "m-"+part, "input %q -> part %q", in, part)
	}
}

func TestAddReference(t *testing.T) {
	t.Run("sets references on a material with no annotations", func(t *testing.T) {
		m := &api.Attestation_Material{}
		addReference(m, "sigcheck-ignored-paths", "sigcheck-third-party-paths")
		assert.Equal(t, "sigcheck-ignored-paths,sigcheck-third-party-paths", m.Annotations[materials.AnnotationMaterialReferences])
	})

	t.Run("appends to and de-duplicates existing references", func(t *testing.T) {
		m := &api.Attestation_Material{Annotations: map[string]string{
			materials.AnnotationMaterialReferences: "existing",
		}}
		addReference(m, "existing", "new")
		assert.Equal(t, "existing,new", m.Annotations[materials.AnnotationMaterialReferences])
	})

	t.Run("no names is a no-op", func(t *testing.T) {
		m := &api.Attestation_Material{}
		addReference(m)
		assert.Empty(t, m.Annotations[materials.AnnotationMaterialReferences])
	})
}

func TestBuildRuntimeInputsNil(t *testing.T) {
	got, err := buildRuntimeInputs(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestBuildRuntimeInputs(t *testing.T) {
	dir := t.TempDir()
	// A single file with two columns reused across inputs.
	path := filepath.Join(dir, "exception.csv")
	require.NoError(t, os.WriteFile(path, []byte("Path,Extra\na.dll,x\nb.dll,y\n"), 0600))

	// Expected joined values for the Path and Extra columns respectively.
	const (
		wantAB = "a.dll\nb.dll"
		wantXY = "x\ny"
	)

	t.Run("unscoped inputs land in Global", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Input: "ignored_paths", Column: "Path", File: path},
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, &policies.RuntimeInputs{
			Global:         map[string]string{"ignored_paths": wantAB},
			Scoped:         map[string]map[string]string{},
			GlobalOverride: map[string]string{},
			ScopedOverride: map[string]map[string]string{},
		}, got)
	})

	t.Run("scoped inputs land under their policy", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Policy: "trusted-binaries-signed", Input: "ignored_paths", Column: "Path", File: path},
			{Policy: "trusted-binaries-vendor-keys", Input: "third_party_paths", Column: "Path", File: path},
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, &policies.RuntimeInputs{
			Global: map[string]string{},
			Scoped: map[string]map[string]string{
				"trusted-binaries-signed":      {"ignored_paths": wantAB},
				"trusted-binaries-vendor-keys": {"third_party_paths": wantAB},
			},
			GlobalOverride: map[string]string{},
			ScopedOverride: map[string]map[string]string{},
		}, got)
	})

	t.Run("repeated scope+input merges additively", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Policy: "p", Input: "ignored_paths", Column: "Path", File: path},
			{Policy: "p", Input: "ignored_paths", Column: "Extra", File: path},
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"ignored_paths": "a.dll\nb.dll\nx\ny"}, got.Scoped["p"])
	})

	t.Run("global and scoped coexist", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Input: "ignored_paths", Column: "Path", File: path},
			{Policy: "p", Input: "ignored_paths", Column: "Extra", File: path},
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"ignored_paths": wantAB}, got.Global)
		assert.Equal(t, map[string]string{"ignored_paths": wantXY}, got.Scoped["p"])
	})

	t.Run("replace-mode file inputs land in the override maps", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Input: "ignored_paths", Column: "Path", File: path, Replace: true},
			{Policy: "p", Input: "third_party_paths", Column: "Extra", File: path, Replace: true},
		}, nil)
		require.NoError(t, err)
		assert.Empty(t, got.Global)
		assert.Empty(t, got.Scoped)
		assert.Equal(t, map[string]string{"ignored_paths": wantAB}, got.GlobalOverride)
		assert.Equal(t, map[string]string{"third_party_paths": wantXY}, got.ScopedOverride["p"])
	})

	t.Run("inline values land in the override maps, last write wins", func(t *testing.T) {
		got, err := buildRuntimeInputs(nil, []*PolicyInput{
			{Input: testInputMinIter, Value: "5"},
			{Input: testInputMinIter, Value: "10"},
			{Policy: testPolicyRadamsa, Input: testInputMinIter, Value: "20"},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{testInputMinIter: "10"}, got.GlobalOverride)
		assert.Equal(t, map[string]string{testInputMinIter: "20"}, got.ScopedOverride[testPolicyRadamsa])
	})

	t.Run("inline --policy-input wins over a file-replace for the same input", func(t *testing.T) {
		// File-replace fills min_iterations from the file column; the inline value
		// is applied afterwards and must win deterministically.
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Input: testInputMinIter, Column: "Path", File: path, Replace: true},
		}, []*PolicyInput{
			{Input: testInputMinIter, Value: "10"},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{testInputMinIter: "10"}, got.GlobalOverride)
	})

	t.Run("append files, replace files and inline values coexist", func(t *testing.T) {
		got, err := buildRuntimeInputs([]*PolicyInputFromFile{
			{Input: "ignored_paths", Column: "Path", File: path},
			{Input: "extra_paths", Column: "Extra", File: path, Replace: true},
		}, []*PolicyInput{
			{Policy: testPolicyRadamsa, Input: testInputMinIter, Value: "10"},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"ignored_paths": wantAB}, got.Global)
		assert.Equal(t, map[string]string{"extra_paths": wantXY}, got.GlobalOverride)
		assert.Equal(t, map[string]string{testInputMinIter: "10"}, got.ScopedOverride[testPolicyRadamsa])
	})
}
