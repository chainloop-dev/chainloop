//
// Copyright 2023-2026 The Chainloop Authors.
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

package v1

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOutput(t *testing.T) {
	artifactBasedMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_SARIF,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{
				Name: "name", Digest: "deadbeef", IsSubject: true, Content: []byte("content"),
			},
		},
	}

	artifactBasedMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true, Content: []byte("content"),
	}

	containerMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
		M: &Attestation_Material_ContainerImage_{
			ContainerImage: &Attestation_Material_ContainerImage{
				Name: "name", Digest: "deadbeef", IsSubject: true,
			},
		},
	}

	containerMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true,
	}

	keyValMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_STRING,
		Id:           "id",
		M: &Attestation_Material_String_{
			String_: &Attestation_Material_KeyVal{
				Value: "value",
			},
		},
	}

	sbomArtifactMaterial := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
		M: &Attestation_Material_SbomArtifact{
			SbomArtifact: &Attestation_Material_SBOMArtifact{
				Artifact: &Attestation_Material_Artifact{
					Name: "name", Digest: "deadbeef", IsSubject: true, Content: []byte("content"),
				},
				MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
					Name: "the-main-component",
				},
			},
		},
	}

	sbomArtifactMaterialWant := &NormalizedMaterialOutput{
		Name: "name", Digest: "deadbeef", IsOutput: true, Content: []byte("content"),
	}

	keyValWant := &NormalizedMaterialOutput{
		Content: []byte("value"),
	}

	testCases := []struct {
		name     string
		material *Attestation_Material
		want     *NormalizedMaterialOutput
		wantErr  string
	}{
		{
			name:    "nil material",
			wantErr: "material not provided",
		},
		{
			name:     "empty material",
			material: &Attestation_Material{},
			wantErr:  "unknown material: MATERIAL_TYPE_UNSPECIFIED",
		},
		{
			name:     "artifact based material",
			material: artifactBasedMaterial,
			want:     artifactBasedMaterialWant,
		},
		{
			name:     "Container image material",
			material: containerMaterial,
			want:     containerMaterialWant,
		},
		{
			name:     "keyval material",
			material: keyValMaterial,
			want:     keyValWant,
		},
		{
			name:     "sbom artifact material",
			material: sbomArtifactMaterial,
			want:     sbomArtifactMaterialWant,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (tc.material).NormalizedOutput()
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetEvaluableContentWithMetadata(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		material  *Attestation_Material
		testField string
	}{
		{
			name: "artifact based material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SARIF,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Content: []byte("{}"),
					},
				},
				InlineCas: true,
			},
		},
		{
			name: "artifact based material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CONTAINER_IMAGE,
				M: &Attestation_Material_ContainerImage_{
					ContainerImage: &Attestation_Material_ContainerImage{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Tag: "latest",
					},
				},
			},
		},
		{
			name: "sbom artifact material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				M: &Attestation_Material_SbomArtifact{
					SbomArtifact: &Attestation_Material_SBOMArtifact{
						Artifact: &Attestation_Material_Artifact{
							Name: "name", Digest: "sha256:deadbeef", IsSubject: true, Content: []byte("{}"),
						},
						MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
							Name: "the-main-component",
						},
					},
				},
				InlineCas: true,
			},
		},
		{
			name: "sbom artifact material not inline",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				M: &Attestation_Material_SbomArtifact{
					SbomArtifact: &Attestation_Material_SBOMArtifact{
						Artifact: &Attestation_Material_Artifact{
							Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						},
						MainComponent: &Attestation_Material_SBOMArtifact_MainComponent{
							Name: "the-main-component",
						},
					},
				},
			},
			filename:  "testdata/sbom.cyclonedx.json",
			testField: "bomFormat",
		},
		{
			name: "cobertura xml material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_COBERTURA_XML,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/cobertura.xml",
			testField: "packages",
		},
		{
			name: "sigcheck csv material",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SYSINTERNALS_SIGCHECK,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/sigcheck-report.csv",
			testField: "elements",
		},
		{
			name: "accesschk text material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_SYSINTERNALS_ACCESSCHK,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("c:\\windows\\system32\\notepad.exe\n  RW BUILTIN\\Administrators\n"),
					},
				},
				InlineCas: true,
			},
			testField: "objects",
		},
		{
			name: "dranzer text material projected to json",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CERTCC_DRANZER,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("Testing COM Object - {11111111-2222-3333-4444-555555555555} Example.WidgetControl\nCOM Object Filename : example.ocx\n"),
					},
				},
				InlineCas: true,
			},
			testField: "objects",
		},
		{
			name: "radamsa report -M log projected to elements",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_REPORT,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/radamsa-meta.txt",
			testField: "elements",
		},
		{
			name: "trufflehog JSONL projected to elements",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
			},
			filename:  "testdata/trufflehog-report.json",
			testField: "elements",
		},
		{
			// metadata-only: the (non-existent) crashes path must NOT be read.
			name: "radamsa crashes metadata only",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_CRASHES,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
					},
				},
				Annotations: map[string]string{"chainloop.material.radamsa.crashes.count": "0"},
			},
			filename: "testdata/this-crashes-file-does-not-exist.tar.gz",
		},
		{
			// inline binary crash content must NOT be parsed as JSON; it is
			// metadata-only regardless of how the content is sourced.
			name: "radamsa crashes inline binary content",
			material: &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_CRASHES,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name: "name", Digest: "sha256:deadbeef", IsSubject: true,
						Content: []byte("\x1f\x8b\x08\x00rawcrashingbytes"),
					},
				},
				InlineCas:   true,
				Annotations: map[string]string{"chainloop.material.radamsa.crashes.count": "1"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := tc.material.GetEvaluableContent(tc.filename)
			assert.NoError(t, err)
			decoder := json.NewDecoder(bytes.NewReader(content))

			var decodedMaterial map[string]interface{}
			err = decoder.Decode(&decodedMaterial)
			assert.NoError(t, err)

			assert.Equal(t, decodedMaterial["chainloop_metadata"].(map[string]any)["name"], "name")

			if tc.testField != "" {
				assert.NotEmpty(t, decodedMaterial[tc.testField])
			}
		})
	}
}

// TestDranzerBundleIsEvaluable guards that a CERTCC_DRANZER material holding an
// archive of per-mode reports projects to an aggregate the existing
// activex-controls-fuzzed policy can evaluate. Recording the archive whole
// without aggregating would hand the policy engine zip bytes, which parse to an
// empty report and make the policy *skip* — a clean-looking false pass.
func TestDranzerBundleIsEvaluable(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_CERTCC_DRANZER,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "dranzer-report", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContent("testdata/dranzer-bundle.zip")
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))

	// The policy reads tool.name and needs run evidence to avoid skipping.
	assert.Equal(t, "dranzer", decoded["tool"].(map[string]any)["name"])
	assert.Equal(t, "96", decoded["tool"].(map[string]any)["version"])

	// failed_count sums across the bundle, so the -t mode's single failure is
	// what makes the gate fire.
	summary := decoded["summary"].(map[string]any)
	assert.EqualValues(t, 1, summary["failed_count"])
	assert.EqualValues(t, 0, summary["hung_count"])
	assert.EqualValues(t, 16, summary["object_count"])

	assert.Len(t, decoded["findings"], 1, "the -t report's crash finding must survive aggregation")

	// The CSV companion is not a report, so only the four modes are listed.
	assert.Len(t, decoded["reports"], 4)
}

// TestRadamsaReportArchiveIsEvaluable guards that a RADAMSA_REPORT material whose
// value is an archive of per-run -M logs projects to input.elements holding the
// records merged across every entry. Recording the archive whole without merging
// here would hand the policy engine zip/tar.gz bytes, which parse to no records,
// so radamsa-min-iterations reads a non-array input.elements and *skips* — a
// clean-looking false pass on a fuzzing-coverage gate.
func TestRadamsaReportArchiveIsEvaluable(t *testing.T) {
	// Two per-run logs, two mutation records each (the seed header line is not a
	// fuzzing iteration and is not counted) => four merged records.
	logA := []byte("seed: 1\nmuta-num: 1, generator: file\nbyte-dec: 1, generator: jump\n")
	logB := []byte("seed: 2\nmuta-num: 2, generator: file\nbyte-dec: 2, generator: jump\n")

	dir := t.TempDir()
	tarGz := filepath.Join(dir, "report.tar.gz")
	writeReportTarGz(t, tarGz, map[string][]byte{"meta_1.log": logA, "meta_2.log": logB})
	zipPath := filepath.Join(dir, "report.zip")
	writeReportZip(t, zipPath, map[string][]byte{"meta_1.log": logA, "meta_2.log": logB})

	for _, tc := range []struct{ name, path string }{
		{"tar.gz", tarGz},
		{"zip", zipPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_RADAMSA_REPORT,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{Name: "fuzz-report", Digest: "sha256:deadbeef"},
				},
			}

			content, err := m.GetEvaluableContent(tc.path)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))

			elements, ok := decoded["elements"].([]any)
			require.True(t, ok, "input.elements must be an array so the gate does not skip")
			assert.Len(t, elements, 4, "mutation records from every archive entry must be merged")
		})
	}
}

func writeReportTarGz(t *testing.T, path string, files map[string][]byte) {
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

func writeReportZip(t *testing.T, path string, files map[string][]byte) {
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

// TestCoberturaEmptyReportIsEvaluable guards the requirement that a legitimate
// empty coverage report (line-rate="NaN", no packages) projects to valid JSON
// the policy engine can evaluate — instead of failing with a NaN marshal error,
// which a policy would surface as a violation. line-rate is null and
// lines-valid is 0, so a policy can guard on lines-valid > 0 and treat the
// report as valid (no violations).
func TestCoberturaEmptyReportIsEvaluable(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_COBERTURA_XML,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "coverage", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContent("testdata/cobertura-empty.xml")
	require.NoError(t, err, "empty report must be evaluable, not error on NaN")

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	assert.Nil(t, decoded["line-rate"], "NaN line-rate must project as null")
	assert.EqualValues(t, 0, decoded["lines-valid"], "lines-valid stays 0 so a policy can detect an empty report")
}

// TestTruffleHogCleanScanIsEvaluable guards that a clean scan (TruffleHog found
// no secrets, leaving a zero-byte file) projects to valid policy input with an
// empty findings list, so a secrets policy sees "no secrets -> no violation"
// rather than an evaluation error.
func TestTruffleHogCleanScanIsEvaluable(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "secrets", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContent("testdata/trufflehog-clean-scan.jsonl")
	require.NoError(t, err, "clean scan must be evaluable")

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	elements, ok := decoded["elements"].([]any)
	require.True(t, ok, "clean scan must project to an elements array")
	assert.Empty(t, elements, "a clean scan has zero findings")
}

// TestGetEvaluableContentRedactedNeverReadsDisk pins the invariant that policies
// are handed exactly the bytes that were stored. For a redacted material the
// original on disk is the one thing they must never see, whatever CAS backend is
// in use: it still holds the secrets that redaction deliberately kept out of the
// evidence store, and a policy is user-authored code that can ship what it reads.
//
// The sanitized copy therefore has to be supplied explicitly. Without it there is
// no safe source, so resolution fails closed rather than falling back to the file.
func TestGetEvaluableContentRedactedNeverReadsDisk(t *testing.T) {
	const (
		// Any distinguishable values work here; this test is about which source
		// the content is read from, not about detection.
		onDiskSecret = "the-unredacted-original"
		inlineSecret = "[REDACTED:aws-access-token]"
		suppliedTag  = "[REDACTED:supplied]"

		onDisk   = `{"secret":"` + onDiskSecret + `"}`
		inline   = `{"secret":"` + inlineSecret + `"}`
		supplied = `{"secret":"` + suppliedTag + `"}`
	)

	diskPath := filepath.Join(t.TempDir(), "session.json")
	require.NoError(t, os.WriteFile(diskPath, []byte(onDisk), 0o600))

	testCases := []struct {
		name        string
		inlineCas   bool
		annotations map[string]string
		path        string
		content     []byte
		wantSecret  string
		wantErr     error
	}{
		{
			name:      "inline and redacted without the sanitized copy fails closed",
			inlineCas: true,
			// The inline bytes ARE the sanitized copy here, so reading them would
			// happen to be correct. It still fails: the caller has not said which
			// content is authoritative, and guessing is what this test forbids.
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			path:        diskPath,
			wantErr:     ErrRedactedContentRequired,
		},
		{
			name:        "non-inline and redacted without the sanitized copy fails closed",
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			path:        diskPath,
			wantErr:     ErrRedactedContentRequired,
		},
		{
			name:        "redacted with neither a path nor the sanitized copy fails closed",
			inlineCas:   true,
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			wantErr:     ErrRedactedContentRequired,
		},
		{
			name: "redacted with an empty but non-nil sanitized copy fails closed",
			// Guards the len() check: a zero-length slice must not slip past into
			// the empty-content fallback and yield an empty policy input.
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			path:        diskPath,
			content:     []byte{},
			wantErr:     ErrRedactedContentRequired,
		},
		{
			name:        "redacted with the sanitized copy evaluates it, not the disk file",
			inlineCas:   true,
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			path:        diskPath,
			content:     []byte(supplied),
			wantSecret:  suppliedTag,
		},
		{
			name:        "redacted with the sanitized copy needs no path at all",
			annotations: map[string]string{AnnotationMaterialRedacted: AnnotationValueTrue},
			content:     []byte(supplied),
			wantSecret:  suppliedTag,
		},
		{
			name: "redaction skipped on purpose still reads from disk",
			// --skip-secret-redaction stores the session exactly as captured, so
			// the disk file and the stored copy are the same bytes. The opt-out is
			// recorded in the attestation and is deliberately not fail-closed.
			annotations: map[string]string{AnnotationMaterialRedactionSkipped: AnnotationValueTrue},
			path:        diskPath,
			wantSecret:  onDiskSecret,
		},
		{
			name:       "unredacted inline reads the inline content as before",
			inlineCas:  true,
			path:       diskPath,
			wantSecret: inlineSecret,
		},
		{
			name:       "unredacted non-inline reads from disk as before",
			path:       diskPath,
			wantSecret: onDiskSecret,
		},
		{
			name:       "supplied content wins over the inline copy even unredacted",
			inlineCas:  true,
			path:       diskPath,
			content:    []byte(supplied),
			wantSecret: suppliedTag,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Attestation_Material{
				MaterialType: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
				InlineCas:    tc.inlineCas,
				Annotations:  tc.annotations,
				M: &Attestation_Material_Artifact_{
					Artifact: &Attestation_Material_Artifact{
						Name:    "session.json",
						Digest:  "sha256:deadbeef",
						Content: []byte(inline),
					},
				},
			}

			content, err := m.GetEvaluableContentFrom(tc.path, tc.content)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			// GetEvaluableContent also injects a chainloop_metadata block, so
			// assert on the field that distinguishes the sources.
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(content, &decoded))
			assert.Equal(t, tc.wantSecret, decoded["secret"])

			if tc.annotations[AnnotationMaterialRedacted] == AnnotationValueTrue {
				assert.NotEqual(t, onDiskSecret, decoded["secret"],
					"the un-redacted original must never reach the policy engine")
			}
		})
	}
}

// TestGetEvaluableContentFromInjectsMetadata pins that supplying the content does
// not bypass the projection the policy engine relies on: the chainloop_metadata
// descriptor is still injected, so a policy can read the redaction annotations
// alongside the sanitized body.
func TestGetEvaluableContentFromInjectsMetadata(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_CHAINLOOP_AI_CODING_SESSION,
		Annotations: map[string]string{
			AnnotationMaterialRedacted:       AnnotationValueTrue,
			AnnotationMaterialRedactionCount: "2",
			AnnotationMaterialRedactionRules: "jwt",
		},
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "session.json", Digest: "sha256:deadbeef"},
		},
	}

	content, err := m.GetEvaluableContentFrom("", []byte(`{"secret":"[REDACTED:jwt]"}`))
	require.NoError(t, err)

	var decoded struct {
		Secret   string `json:"secret"`
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"chainloop_metadata"`
	}
	require.NoError(t, json.Unmarshal(content, &decoded))

	assert.Equal(t, "[REDACTED:jwt]", decoded.Secret)
	assert.Equal(t, AnnotationValueTrue, decoded.Metadata.Annotations[AnnotationMaterialRedacted])
	assert.Equal(t, "2", decoded.Metadata.Annotations[AnnotationMaterialRedactionCount])
	assert.Equal(t, "jwt", decoded.Metadata.Annotations[AnnotationMaterialRedactionRules])
}

// TestTruffleHogCleanScanIsEvaluableInline is the inline counterpart of
// TestTruffleHogCleanScanIsEvaluable. It pins down that the canonical empty
// content substituted for a zero-byte report projects the same either way, so
// that resolving a redacted material's content differently cannot regress it.
func TestTruffleHogCleanScanIsEvaluableInline(t *testing.T) {
	m := &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_TRUFFLEHOG_JSON,
		InlineCas:    true,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{
				Name:    "secrets",
				Digest:  "sha256:deadbeef",
				Content: []byte("[]"),
			},
		},
	}

	content, err := m.GetEvaluableContent("testdata/trufflehog-clean-scan.jsonl")
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	elements, ok := decoded["elements"].([]any)
	require.True(t, ok, "clean scan must project to an elements array")
	assert.Empty(t, elements)
}

// pitestMaterial builds an artifact material of kind PITEST_XML for
// projection tests.
func pitestMaterial() *Attestation_Material {
	return &Attestation_Material{
		MaterialType: schemaapi.CraftingSchema_Material_PITEST_XML,
		M: &Attestation_Material_Artifact_{
			Artifact: &Attestation_Material_Artifact{Name: "mutations", Digest: "sha256:deadbeef"},
		},
	}
}

// pitestMutations decodes the evaluable content of a PIT report and returns
// its mutation records.
func pitestMutations(t *testing.T, path string) (map[string]any, []any) {
	t.Helper()

	content, err := pitestMaterial().GetEvaluableContent(path)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(bytes.NewReader(content)).Decode(&decoded))
	mutations, ok := decoded["mutations"].([]any)
	require.True(t, ok, "PIT report must project to a mutations array")
	return decoded, mutations
}

// TestPitestReportIsEvaluable guards the XML-to-JSON projection of a standard
// PIT report: the root partial flag is preserved and every mutation record is
// projected with its own fields.
func TestPitestReportIsEvaluable(t *testing.T) {
	decoded, mutations := pitestMutations(t, "testdata/pitest.xml")
	assert.Equal(t, true, decoded["partial"], "root partial attribute must be preserved")
	assert.Len(t, mutations, 3, "every mutation record must be projected")
}

// TestPitestSurvivedMutationKeepsStatus guards that a SURVIVED mutant keeps
// detected=false and status=SURVIVED along with its source location, mutator,
// indexes, blocks and description, so a policy can single out surviving
// covered mutants.
func TestPitestSurvivedMutationKeepsStatus(t *testing.T) {
	_, mutations := pitestMutations(t, "testdata/pitest.xml")

	var survived map[string]any
	for _, m := range mutations {
		mutation := m.(map[string]any)
		if mutation["status"] == "SURVIVED" {
			survived = mutation
			break
		}
	}
	require.NotNil(t, survived, "fixture must contain a SURVIVED mutation")
	assert.Equal(t, false, survived["detected"])
	assert.Equal(t, "SURVIVED", survived["status"])
	assert.Equal(t, "PetController.java", survived["sourceFile"])
	assert.Equal(t, "org.springframework.samples.petclinic.owner.PetController", survived["mutatedClass"])
	assert.NotEmpty(t, survived["mutatedMethod"])
	assert.NotEmpty(t, survived["methodDescription"])
	assert.NotZero(t, survived["lineNumber"])
	assert.NotEmpty(t, survived["mutator"])
	assert.NotEmpty(t, survived["indexes"])
	assert.NotEmpty(t, survived["blocks"])
	assert.NotEmpty(t, survived["description"])
}

// TestPitestNoCoverageKeepsStatusDistinct guards that a NO_COVERAGE mutant is
// not collapsed into the same signal as a surviving covered mutant: it keeps
// status=NO_COVERAGE and numberOfTestsRun=0.
func TestPitestNoCoverageKeepsStatusDistinct(t *testing.T) {
	_, mutations := pitestMutations(t, "testdata/pitest.xml")

	var noCoverage map[string]any
	for _, m := range mutations {
		mutation := m.(map[string]any)
		if mutation["status"] == "NO_COVERAGE" {
			noCoverage = mutation
			break
		}
	}
	require.NotNil(t, noCoverage, "fixture must contain a NO_COVERAGE mutation")
	assert.Equal(t, "NO_COVERAGE", noCoverage["status"])
	assert.Equal(t, false, noCoverage["detected"])
	assert.EqualValues(t, 0, noCoverage["numberOfTestsRun"])
}

// TestPitestFullMutationMatrixIsEvaluable guards that a report generated with
// PIT's fullMutationMatrix option preserves its killingTests, succeedingTests
// and coveringTests lists (pipe-delimited, kept unsplit) instead of the
// standard killingTest field.
func TestPitestFullMutationMatrixIsEvaluable(t *testing.T) {
	_, mutations := pitestMutations(t, "testdata/pitest-full-matrix.xml")
	require.Len(t, mutations, 1)

	mutation := mutations[0].(map[string]any)
	assert.Equal(t,
		"org.springframework.samples.petclinic.owner.PetControllerTests.processCreationFormSuccess()|org.springframework.samples.petclinic.owner.PetControllerTests.processCreationFormError()",
		mutation["killingTests"])
	assert.Equal(t,
		"org.springframework.samples.petclinic.owner.PetControllerTests.processUpdateForm()",
		mutation["succeedingTests"])
	assert.Equal(t,
		"org.springframework.samples.petclinic.owner.PetControllerTests.processCreationFormSuccess()|org.springframework.samples.petclinic.owner.PetControllerTests.processUpdateForm()",
		mutation["coveringTests"])
	assert.NotContains(t, mutation, "killingTest", "full-matrix reports must not manufacture the standard killingTest field")
}

// TestPitestInvalidReportIsNotEvaluable guards that a non-PIT XML report
// fails the projection instead of producing an empty policy input.
func TestPitestInvalidReportIsNotEvaluable(t *testing.T) {
	_, err := pitestMaterial().GetEvaluableContent("testdata/cobertura.xml")
	require.ErrorContains(t, err, "invalid PIT report file")
}
