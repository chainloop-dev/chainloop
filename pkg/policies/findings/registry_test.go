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

package findings

import (
	"testing"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestIsValidFindingType(t *testing.T) {
	tests := []struct {
		name        string
		findingType string
		want        bool
	}{
		{"vulnerability", "VULNERABILITY", true},
		{"sast", "SAST", true},
		{"license_violation", "LICENSE_VIOLATION", true},
		{"unknown", "UNKNOWN", false},
		{"empty", "", false},
		{"lowercase", "vulnerability", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsValidFindingType(tc.findingType))
		})
	}
}

func TestValidateFinding(t *testing.T) {
	tests := []struct {
		name        string
		findingType string
		raw         map[string]any
		wantErr     string
		checkFn     func(t *testing.T, msg interface{})
	}{
		{
			name:        "valid vulnerability finding",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":       "Found CVE-2024-1234",
				"external_id":   "CVE-2024-1234",
				"package_purl":  "pkg:golang/example.com/lib@v1.0.0",
				"severity":      "CRITICAL",
				"cvss_v3_score": 9.8,
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicyVulnerabilityFinding)
				require.True(t, ok)
				assert.Equal(t, "Found CVE-2024-1234", f.GetMessage())
				assert.Equal(t, "CVE-2024-1234", f.GetExternalId())
				assert.Equal(t, "pkg:golang/example.com/lib@v1.0.0", f.GetPackagePurl())
				assert.Equal(t, "CRITICAL", f.GetSeverity())
				assert.InDelta(t, 9.8, f.GetCvssV3Score(), 0.01)
			},
		},
		{
			name:        "valid vulnerability finding with description",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":      "Found CVE-2024-5678",
				"external_id":  "CVE-2024-5678",
				"package_purl": "pkg:golang/example.com/lib@v2.0.0",
				"severity":     "HIGH",
				"description":  "A buffer overflow vulnerability in the parsing module allows remote code execution.",
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicyVulnerabilityFinding)
				require.True(t, ok)
				assert.Equal(t, "Found CVE-2024-5678", f.GetMessage())
				assert.Equal(t, "CVE-2024-5678", f.GetExternalId())
				assert.Equal(t, "HIGH", f.GetSeverity())
				assert.Equal(t, "A buffer overflow vulnerability in the parsing module allows remote code execution.", f.GetDescription())
			},
		},
		{
			name:        "valid vulnerability finding with fixed_version",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":       "Found CVE-2024-5678",
				"external_id":   "CVE-2024-5678",
				"package_purl":  "pkg:golang/example.com/lib@v1.0.0",
				"severity":      "HIGH",
				"fixed_version": "1.0.1",
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicyVulnerabilityFinding)
				require.True(t, ok)
				assert.Equal(t, "Found CVE-2024-5678", f.GetMessage())
				assert.Equal(t, "CVE-2024-5678", f.GetExternalId())
				assert.Equal(t, "pkg:golang/example.com/lib@v1.0.0", f.GetPackagePurl())
				assert.Equal(t, "HIGH", f.GetSeverity())
				assert.Equal(t, "1.0.1", f.GetFixedVersion())
			},
		},
		{
			name:        "vulnerability finding missing required field",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":     "Found CVE-2024-1234",
				"external_id": "CVE-2024-1234",
				// missing package_purl and severity
			},
			wantErr: "finding validation failed",
		},
		{
			name:        "vulnerability finding with out-of-range CVSS score",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":       "Found CVE-2024-1234",
				"external_id":   "CVE-2024-1234",
				"package_purl":  "pkg:golang/example.com/lib@v1.0.0",
				"severity":      "CRITICAL",
				"cvss_v3_score": 15.0,
			},
			wantErr: "finding validation failed",
		},
		{
			name:        "valid SAST finding",
			findingType: "SAST",
			raw: map[string]any{
				"message":  "SQL injection in handler",
				"rule_id":  "java:S1234",
				"severity": "HIGH",
				"location": "src/main/Handler.java",
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicySASTFinding)
				require.True(t, ok)
				assert.Equal(t, "SQL injection in handler", f.GetMessage())
				assert.Equal(t, "java:S1234", f.GetRuleId())
				assert.Equal(t, "HIGH", f.GetSeverity())
				assert.Equal(t, "src/main/Handler.java", f.GetLocation())
				assert.Nil(t, f.SeverityScore)
			},
		},
		{
			name:        "valid SAST finding with severity_score",
			findingType: "SAST",
			raw: map[string]any{
				"message":        "SQL injection in handler",
				"rule_id":        "java:S1234",
				"severity":       "HIGH",
				"location":       "src/main/Handler.java",
				"severity_score": 7.5,
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicySASTFinding)
				require.True(t, ok)
				require.NotNil(t, f.SeverityScore)
				assert.InDelta(t, 7.5, *f.SeverityScore, 1e-9)
			},
		},
		{
			name:        "SAST finding missing required field",
			findingType: "SAST",
			raw: map[string]any{
				"message": "SQL injection",
				// missing rule_id, severity, location
			},
			wantErr: "finding validation failed",
		},
		{
			name:        "valid license violation finding",
			findingType: "LICENSE_VIOLATION",
			raw: map[string]any{
				"message":        "Banned license GPL-3.0",
				"component_name": "lodash",
				"license_id":     "GPL-3.0",
				"package_purl":   "pkg:npm/lodash@4.17.21",
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicyLicenseViolationFinding)
				require.True(t, ok)
				assert.Equal(t, "Banned license GPL-3.0", f.GetMessage())
				assert.Equal(t, "lodash", f.GetComponentName())
				assert.Equal(t, "GPL-3.0", f.GetLicenseId())
			},
		},
		{
			name:        "vulnerability finding with unknown field is accepted",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":      "Found CVE-2024-9999",
				"external_id":  "CVE-2024-9999",
				"package_purl": "pkg:golang/example.com/lib@v3.0.0",
				"severity":     "LOW",
				"future_field": "some value from a newer policy",
			},
			checkFn: func(t *testing.T, msg interface{}) {
				t.Helper()
				f, ok := msg.(*v1.PolicyVulnerabilityFinding)
				require.True(t, ok)
				assert.Equal(t, "CVE-2024-9999", f.GetExternalId())
			},
		},
		{
			name:        "unknown finding type",
			findingType: "UNKNOWN",
			raw:         map[string]any{"message": "test"},
			wantErr:     "unknown finding type",
		},
		{
			name:        "invalid field type in raw data",
			findingType: "VULNERABILITY",
			raw: map[string]any{
				"message":      123, // should be string
				"external_id":  "CVE-2024-1234",
				"package_purl": "pkg:golang/example.com/lib@v1.0.0",
				"severity":     "HIGH",
			},
			wantErr: "does not match VULNERABILITY schema",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ValidateFinding(tc.findingType, tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, msg)
			if tc.checkFn != nil {
				tc.checkFn(t, msg)
			}
		})
	}
}

func TestSetViolationFinding(t *testing.T) {
	sastSeverityScore := 4.0
	tests := []struct {
		name        string
		findingType string
		finding     proto.Message
		wantErr     string
		checkFn     func(t *testing.T, v *v1.PolicyEvaluation_Violation)
	}{
		{
			name:        "set vulnerability finding",
			findingType: "VULNERABILITY",
			finding: &v1.PolicyVulnerabilityFinding{
				Message:     "test",
				ExternalId:  "CVE-2024-1",
				PackagePurl: "pkg:npm/foo@1.0",
				Severity:    "HIGH",
			},
			checkFn: func(t *testing.T, v *v1.PolicyEvaluation_Violation) {
				t.Helper()
				f := v.GetVulnerability()
				require.NotNil(t, f)
				assert.Equal(t, "CVE-2024-1", f.GetExternalId())
			},
		},
		{
			name:        "set SAST finding",
			findingType: "SAST",
			finding: &v1.PolicySASTFinding{
				Message:       "test",
				RuleId:        "go-sec:G101",
				Severity:      "MEDIUM",
				Location:      "main.go",
				SeverityScore: &sastSeverityScore,
			},
			checkFn: func(t *testing.T, v *v1.PolicyEvaluation_Violation) {
				t.Helper()
				f := v.GetSast()
				require.NotNil(t, f)
				assert.Equal(t, "go-sec:G101", f.GetRuleId())
				require.NotNil(t, f.SeverityScore)
				assert.InDelta(t, 4.0, *f.SeverityScore, 1e-9)
			},
		},
		{
			name:        "set license violation finding",
			findingType: "LICENSE_VIOLATION",
			finding: &v1.PolicyLicenseViolationFinding{
				Message:       "test",
				ComponentName: "foo",
				LicenseId:     "MIT",
			},
			checkFn: func(t *testing.T, v *v1.PolicyEvaluation_Violation) {
				t.Helper()
				f := v.GetLicenseViolation()
				require.NotNil(t, f)
				assert.Equal(t, "foo", f.GetComponentName())
			},
		},
		{
			name:        "unknown finding type",
			findingType: "UNKNOWN",
			finding:     &v1.PolicyVulnerabilityFinding{},
			wantErr:     "unknown finding type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			violation := &v1.PolicyEvaluation_Violation{
				Subject: "test-policy",
				Message: "test violation",
			}

			err := SetViolationFinding(violation, tc.findingType, tc.finding)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			if tc.checkFn != nil {
				tc.checkFn(t, violation)
			}
		})
	}
}
