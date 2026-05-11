//
// Copyright 2024 The Chainloop Authors.
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

package cmd

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	attv1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type workflowRunDescribeSuite struct {
	suite.Suite

	run *action.WorkflowRunItemFull
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(workflowRunDescribeSuite))
}

func (s *workflowRunDescribeSuite) SetupTest() {
	s.run = &action.WorkflowRunItemFull{
		Attestation: &action.WorkflowRunAttestationItem{
			Envelope: &dsse.Envelope{
				PayloadType: "application/vnd.in-toto+json",
				Payload:     base64.StdEncoding.EncodeToString([]byte("hello")),
				Signatures:  nil,
			},
		},
	}
}

func TestViolationSummary(t *testing.T) {
	tests := []struct {
		name      string
		violation *action.PolicyViolation
		want      string
	}{
		{
			name: "vulnerability with severity, package, no fix",
			violation: &action.PolicyViolation{
				Vulnerability: &attv1.PolicyVulnerabilityFinding{
					ExternalId:  "CVE-2026-5450",
					Severity:    "critical",
					PackagePurl: "pkg:deb/debian/libc6@2.41-12%2Bdeb13u2",
				},
			},
			want: "CVE-2026-5450 (CRITICAL) libc6@2.41-12+deb13u2 [no fix]",
		},
		{
			name: "vulnerability with fix version",
			violation: &action.PolicyViolation{
				Vulnerability: &attv1.PolicyVulnerabilityFinding{
					ExternalId:   "CVE-2024-9999",
					Severity:     "high",
					PackagePurl:  "pkg:golang/example.com/lib@v1.0.0",
					FixedVersion: "1.0.1",
				},
			},
			want: "CVE-2024-9999 (HIGH) lib@v1.0.0 [fix: 1.0.1]",
		},
		{
			name: "suppressed vulnerability gets assessment status inline",
			violation: &action.PolicyViolation{
				Suppress: true,
				Vulnerability: &attv1.PolicyVulnerabilityFinding{
					ExternalId:  "CVE-2018-XXXX",
					Severity:    "low",
					PackagePurl: "pkg:deb/debian/libc6@2.41",
					Assessment: &attv1.PolicyAssessmentResult{
						EffectiveStatus: "ASSESSMENT_STATUS_NOT_AFFECTED",
					},
				},
			},
			want: "CVE-2018-XXXX (LOW, NOT_AFFECTED) libc6@2.41 [no fix]",
		},
		{
			name: "SAST with location and line number",
			violation: &action.PolicyViolation{
				Sast: &attv1.PolicySASTFinding{
					RuleId:     "go-sec:G101",
					Severity:   "medium",
					Location:   "internal/auth.go",
					LineNumber: 42,
				},
			},
			want: "go-sec:G101 (MEDIUM) at internal/auth.go:42",
		},
		{
			name: "license violation with component and version",
			violation: &action.PolicyViolation{
				LicenseViolation: &attv1.PolicyLicenseViolationFinding{
					LicenseId:        "GPL-3.0",
					ComponentName:    "lodash",
					ComponentVersion: "4.17.21",
				},
			},
			want: "GPL-3.0 — lodash@4.17.21",
		},
		{
			name: "unstructured policy uses first line of message",
			violation: &action.PolicyViolation{
				Subject: "repo",
				Message: "missing VEX material",
			},
			want: "missing VEX material",
		},
		{
			name: "unstructured suppressed without assessment falls back to literal tag",
			violation: &action.PolicyViolation{
				Suppress: true,
				Subject:  "repo",
				Message:  "missing VEX material",
			},
			want: "missing VEX material (suppressed)",
		},
		{
			name: "unstructured multi-line message keeps only first line",
			violation: &action.PolicyViolation{
				Subject: "x",
				Message: "first line\nsecond line\nthird line",
			},
			want: "first line",
		},
		{
			name: "empty message falls back to subject",
			violation: &action.PolicyViolation{
				Subject: "missing-tag-annotation",
				Message: "",
			},
			want: "missing-tag-annotation",
		},
		{
			name: "vulnerability with purl qualifiers strips them",
			violation: &action.PolicyViolation{
				Vulnerability: &attv1.PolicyVulnerabilityFinding{
					ExternalId:  "CVE-2024-1",
					Severity:    "high",
					PackagePurl: "pkg:deb/debian/libc6@2.41?arch=amd64",
				},
			},
			want: "CVE-2024-1 (HIGH) libc6@2.41 [no fix]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, violationSummary(tc.violation))
		})
	}
}

func (s *workflowRunDescribeSuite) TestOutputTypePayload() {
	flagOutputFormat = formatPayloadPAE
	expected := "DSSEv1 28 application/vnd.in-toto+json 5 hello"

	buf := new(bytes.Buffer)
	err := encodeAttestationOutput(s.run, buf)
	s.NoError(err)

	s.Require().NoError(err)
	s.Equal(expected, buf.String())
}
