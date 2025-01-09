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

package policies

import (
	"context"
	"io/fs"
	"os"
	"testing"

	v12 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"
)

func (s *testSuite) TestVerifyAttestations() {
	cases := []struct {
		name       string
		schema     *v12.CraftingSchema
		statement  string
		npolicies  int
		violations int
		wantErr    error
	}{
		{
			name: "happy path, test attestation properties",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
					},
				},
			},
			statement: "testdata/statement.json",
			npolicies: 1,
		},
		{
			name:       "wrong runner",
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement_gitlab.json",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
					},
				},
			},
		},
		{
			name:       "missing runner",
			npolicies:  1,
			violations: 1,
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow.yaml"}},
					},
				},
			},
			statement: "testdata/statement_missing_runner.json",
		},
		{
			name:    "wrong policy",
			wantErr: &fs.PathError{},
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/wrong_policy.yaml"}},
					},
				},
			},
			statement: "testdata/statement.json",
		},
		{
			name:    "missing rego policy",
			wantErr: &fs.PathError{},
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/missing_rego.yaml"}},
					},
				},
			},
			statement: "testdata/statement.json",
		},
		{
			name: "embedded rego policy",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow_embedded.yaml"}},
					},
				},
			},
			statement: "testdata/statement.json",
			npolicies: 1,
		},
		{
			name: "embedded rego policy violations",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow_embedded.yaml"}},
					},
				},
			},
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement_missing_runner.json",
		},
		{
			name: "multiple policies",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/workflow_embedded.yaml"}},
						{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/materials.yaml"}},
					},
				},
			},
			npolicies:  2,
			violations: 1,
			statement:  "testdata/statement.json",
		},
		{
			name: "with arguments, no violations",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
							With:   map[string]string{"email": "devel@chainloop.dev"},
						},
					},
				},
			},
			npolicies:  1,
			violations: 0,
			statement:  "testdata/statement.json",
		},
		{
			name: "with arguments, violations",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
							With:   map[string]string{"email": "foobar@chainloop.dev"},
						},
					},
				},
			},
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement.json",
		},
		{
			name: "with array argument, multiline string",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
							With: map[string]string{"email_array": `
								foobar@chainloop.dev
								foobaz@chainloop.dev`},
						},
					},
				},
			},
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement.json",
		},
		{
			name: "with array argument, csv string",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
							With:   map[string]string{"email_array": "foobar@chainloop.dev,foobaz@chainloop.dev"},
						},
					},
				},
			},
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement.json",
		},
		{
			name: "with array argument, malformed csv string",
			schema: &v12.CraftingSchema{
				Policies: &v12.Policies{
					Attestation: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
							With:   map[string]string{"email_array": ",,foobar@chainloop.dev,foobaz@chainloop.dev,,"},
						},
					},
				},
			},
			npolicies:  1,
			violations: 1,
			statement:  "testdata/statement.json",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			verifier := NewPolicyVerifier(tc.schema, nil, &s.logger)
			statement := loadStatement(tc.statement, &s.Suite)

			res, err := verifier.VerifyStatement(context.TODO(), statement)
			if tc.wantErr != nil {
				// #nosec G601
				s.ErrorAs(err, &tc.wantErr)
				return
			}
			s.Require().NoError(err)
			s.Len(res, tc.npolicies)
			if tc.npolicies > 0 {
				violations := 0
				for _, pol := range res {
					violations += len(pol.Violations)
				}
				s.Equal(tc.violations, violations)
			}
		})
	}
}

func (s *testSuite) TestProviderParts() {
	testCases := []struct {
		ref    string
		prov   string
		name   string
		org    string
		digest string
	}{
		{
			ref:  "chainloop://cyclonedx-freshness",
			prov: "",
			org:  "",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "chainloop://provider:cyclonedx-freshness",
			prov: "provider",
			org:  "",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "provider:cyclonedx-freshness",
			prov: "provider",
			org:  "",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "cyclonedx-freshness",
			prov: "",
			org:  "",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "chainloop://builtin:myorg/cyclonedx-freshness",
			prov: "builtin",
			org:  "myorg",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "chainloop://myorg/cyclonedx-freshness",
			prov: "",
			org:  "myorg",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "builtin:myorg/cyclonedx-freshness",
			prov: "builtin",
			org:  "myorg",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "myorg/cyclonedx-freshness",
			prov: "",
			org:  "myorg",
			name: "cyclonedx-freshness",
		},
		{
			ref:  "myorg/cyclonedx-freshness@sha256:123123123",
			org:  "myorg",
			name: "cyclonedx-freshness@sha256:123123123",
		},
		{
			ref:  "builtin:myorg/cyclonedx-freshness@sha256:123123123",
			prov: "builtin",
			org:  "myorg",
			name: "cyclonedx-freshness@sha256:123123123",
		},
		{
			ref:  "chainloop://builtin:myorg/cyclonedx-freshness@sha256:123123123",
			prov: "builtin",
			org:  "myorg",
			name: "cyclonedx-freshness@sha256:123123123",
		},
		{
			ref:  "chainloop://myorg/cyclonedx-freshness@sha256:123123123",
			prov: "",
			org:  "myorg",
			name: "cyclonedx-freshness@sha256:123123123",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ref := ProviderParts(tc.ref)
			s.Equal(tc.prov, ref.Provider)
			s.Equal(tc.name, ref.Name)
			s.Equal(tc.org, ref.OrgName)
		})
	}
}

func (s *testSuite) TestIsProviderScheme() {
	testCases := []struct {
		ref  string
		want bool
	}{
		{
			ref:  "chainloop://cyclonedx-freshness",
			want: true,
		},
		{
			ref:  "chainloop://provider/cyclonedx-freshness",
			want: true,
		},
		{
			ref:  "file://mypolicy.yaml",
			want: false,
		},
		{
			ref:  "https://myserver/mypolicy.yaml",
			want: false,
		},
		{
			ref:  "cyclonedx-freshness",
			want: true,
		},
		{
			ref:  "provider/cyclonedx-freshness",
			want: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.ref, func() {
			s.Equal(tc.want, IsProviderScheme(tc.ref))
		})
	}
}

func (s *testSuite) TestArgumentsInViolations() {
	schema := &v12.CraftingSchema{
		Policies: &v12.Policies{
			Attestation: []*v12.PolicyAttachment{
				{
					Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/with_arguments.yaml"},
					With:   map[string]string{"email_array": "foobar@chainloop.dev"},
				},
			},
		},
	}

	s.Run("arguments in violations", func() {
		verifier := NewPolicyVerifier(schema, nil, &s.logger)
		statement := loadStatement("testdata/statement.json", &s.Suite)

		res, err := verifier.VerifyStatement(context.TODO(), statement)
		s.NoError(err)
		s.Len(res, 1)
		s.Equal(map[string]string{"email_array": "foobar@chainloop.dev"}, res[0].GetWith())
	})
}

func (s *testSuite) TestMaterialSelectionCriteria() {
	attNoFilterPolicyTyped := &v12.PolicyAttachment{
		Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
	}
	attFilteredPolicyTyped := &v12.PolicyAttachment{
		Policy:   &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
		Selector: &v12.PolicyAttachment_MaterialSelector{Name: "sbom"},
	}
	attFilteredPolicyNotTyped := &v12.PolicyAttachment{
		Policy:   &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft_not_typed.yaml"},
		Selector: &v12.PolicyAttachment_MaterialSelector{Name: "custom-material"},
	}
	attMultikind := &v12.PolicyAttachment{Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/multi-kind.yaml"}}

	testcases := []struct {
		name     string
		policies []*v12.PolicyAttachment
		material *v1.Attestation_Material
		wantErr  bool
		result   int
	}{
		{
			name:     "attachment with no filter, policy with type, matched material",
			policies: []*v12.PolicyAttachment{attNoFilterPolicyTyped},
			material: &v1.Attestation_Material{MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON},
			result:   1,
		},
		{
			name:     "attachment with no filter, policy with type, non matched material",
			policies: []*v12.PolicyAttachment{attNoFilterPolicyTyped},
			material: &v1.Attestation_Material{MaterialType: v12.CraftingSchema_Material_SBOM_CYCLONEDX_JSON},
			result:   0,
		},
		{
			name:     "attachment with filter, policy with type, matched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyTyped},
			material: &v1.Attestation_Material{
				Id: "sbom",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON},
			result: 1,
		},
		{
			name:     "attachment with filter, policy with type, unmatched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyTyped},
			material: &v1.Attestation_Material{
				Id: "not-the-sbom-you-expect",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON},
			result: 0,
		},
		{
			name:     "attachment with no filter, policy without type, matched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyNotTyped},
			material: &v1.Attestation_Material{
				Id: "custom-material",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_ATTESTATION},
			result: 1,
		},
		{
			name:     "attachment with no filter, policy without type, unmatched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyNotTyped},
			material: &v1.Attestation_Material{
				Id: "not-the-material-you-expect",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_ATTESTATION},
			result: 0,
		},
		{
			name:     "multi-kind policy",
			policies: []*v12.PolicyAttachment{attMultikind},
			material: &v1.Attestation_Material{
				Id: "custom-material",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_ATTESTATION,
			},
			result: 1,
		},
		{
			name:     "multi-kind policy with no matches",
			policies: []*v12.PolicyAttachment{attMultikind},
			material: &v1.Attestation_Material{
				Id: "custom-material",
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{},
				},
				MaterialType: v12.CraftingSchema_Material_SARIF,
			},
			result: 0,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			schema := &v12.CraftingSchema{
				Policies: &v12.Policies{
					Materials: tc.policies,
				},
			}
			pv := NewPolicyVerifier(schema, nil, &s.logger)
			atts, err := pv.requiredPoliciesForMaterial(context.TODO(), tc.material)
			s.Require().NoError(err)
			s.Require().Len(atts, tc.result)
		})
	}
}

func (s *testSuite) TestValidInlineMaterial() {
	content, err := os.ReadFile("testdata/sbom-spdx.json")
	s.Require().NoError(err)

	schema := &v12.CraftingSchema{
		Materials: []*v12.CraftingSchema_Material{
			{
				Name: "sbom",
				Type: v12.CraftingSchema_Material_SBOM_SPDX_JSON,
			},
		},
		Policies: &v12.Policies{
			Materials: []*v12.PolicyAttachment{
				{
					Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
				},
			},
			Attestation: nil,
		},
	}
	material := &v1.Attestation_Material{
		M: &v1.Attestation_Material_Artifact_{Artifact: &v1.Attestation_Material_Artifact{
			Content: content,
		}},
		MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON,
		InlineCas:    true,
	}

	verifier := NewPolicyVerifier(schema, nil, &s.logger)

	res, err := verifier.VerifyMaterial(context.TODO(), material, "")
	s.Require().NoError(err)
	s.Len(res, 1)
	s.Equal("made-with-syft", res[0].Name)
	s.Equal(v12.CraftingSchema_Material_SBOM_SPDX_JSON, res[0].GetType())
	s.Len(res[0].Violations, 0)
}

func (s *testSuite) TestInvalidInlineMaterial() {
	schema := &v12.CraftingSchema{
		Materials: []*v12.CraftingSchema_Material{
			{
				Name: "sbom",
				Type: v12.CraftingSchema_Material_SBOM_SPDX_JSON,
			},
		},
		Policies: &v12.Policies{
			Materials: []*v12.PolicyAttachment{
				{
					Policy: &v12.PolicyAttachment_Ref{Ref: "file://testdata/sbom_syft.yaml"},
				},
			},
			Attestation: nil,
		},
	}
	material := &v1.Attestation_Material{
		M: &v1.Attestation_Material_Artifact_{Artifact: &v1.Attestation_Material_Artifact{
			Content: []byte(`{"this": { "is": "not", "a": "sbom"}}`),
		}},
		MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON,
		InlineCas:    true,
	}

	verifier := NewPolicyVerifier(schema, nil, &s.logger)

	res, err := verifier.VerifyMaterial(context.TODO(), material, "")
	s.Require().NoError(err)
	s.Len(res, 1)
	s.Equal("made-with-syft", res[0].Name)
	s.Len(res[0].Violations, 1)
	s.Equal("made-with-syft", res[0].Violations[0].Subject)
	s.Equal("Not made with syft", res[0].Violations[0].Message)
}

func (s *testSuite) TestLoadPolicySpec() {
	var cases = []struct {
		name             string
		attachment       *v12.PolicyAttachment
		wantErr          bool
		expectedName     string
		expectedDesc     string
		expectedCategory string
		expectedRef      *PolicyDescriptor
	}{
		{
			name:       "missing policy",
			attachment: &v12.PolicyAttachment{},
			wantErr:    true,
		},
		{
			name: "by file ref",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Ref{
					Ref: "file://testdata/sbom_syft.yaml",
				},
			},
			expectedName:     "made-with-syft",
			expectedDesc:     "This policy checks that the SPDX SBOM was created with syft",
			expectedCategory: "SBOM",
			expectedRef: &PolicyDescriptor{
				URI:    "file://testdata/sbom_syft.yaml",
				Digest: "sha256:81b7fbe4c6ef2182fd042a28fa7f3b3971879d18994147cb812b8fe87a4e04e5",
			},
		},
		{
			name: "by file ref with valid digest",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Ref{
					Ref: "file://testdata/sbom_syft.yaml@sha256:81b7fbe4c6ef2182fd042a28fa7f3b3971879d18994147cb812b8fe87a4e04e5",
				},
			},
			expectedName: "made-with-syft",
			expectedRef: &PolicyDescriptor{
				URI:    "file://testdata/sbom_syft.yaml",
				Digest: "sha256:81b7fbe4c6ef2182fd042a28fa7f3b3971879d18994147cb812b8fe87a4e04e5",
			},
		},
		{
			name: "by file ref with invalid digest",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Ref{
					Ref: "file://testdata/sbom_syft.yaml@sha256:deadbeef",
				},
			},
			wantErr: true,
		},
		{
			name: "embedded invalid",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Embedded{
					Embedded: &v12.Policy{
						ApiVersion: "",
						Kind:       "",
						Metadata:   &v12.Metadata{Name: "my-policy"},
						Spec:       nil,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "embedded valid",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Embedded{
					Embedded: &v12.Policy{
						ApiVersion: "workflowcontract.chainloop.dev/v1",
						Kind:       "Policy",
						Metadata:   &v12.Metadata{Name: "my-policy"},
						Spec: &v12.PolicySpec{
							Source: &v12.PolicySpec_Path{Path: "file.rego"},
							Type:   v12.CraftingSchema_Material_OPENVEX,
						},
					},
				},
			},
			expectedName: "my-policy",
			expectedRef:  nil,
		},
	}

	verifier := NewPolicyVerifier(nil, nil, &s.logger)
	for _, tc := range cases {
		s.Run(tc.name, func() {
			p, gotRef, err := verifier.loadPolicySpec(context.TODO(), tc.attachment)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.expectedName, p.Metadata.Name)
			if tc.expectedDesc != "" {
				s.Equal(tc.expectedDesc, p.Metadata.Description)
			}

			if tc.expectedCategory != "" {
				s.Equal(tc.expectedCategory, p.Metadata.Annotations["category"])
			}

			s.Equal(tc.expectedRef, gotRef)
		})
	}
}

func (s *testSuite) TestLoader() {
	cases := []struct {
		name     string
		ref      string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "no protocol",
			ref:      "remote-policy.yaml",
			expected: &ChainloopLoader{},
		},
		{
			name:     "file ref",
			ref:      "file://local-policy.yaml",
			expected: &FileLoader{},
		},
		{
			name:     "http ref",
			ref:      "https://myhost/policy.yaml",
			expected: &HTTPSLoader{},
		},
		{
			name:    "invalid ref",
			ref:     "env://environmentvar",
			wantErr: true,
		},
		{
			name:     "chainloop ref",
			ref:      "chainloop://provider/policy",
			expected: &ChainloopLoader{},
		},
		{
			name:    "empty ref",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			v := NewPolicyVerifier(nil, nil, &s.logger)
			att := &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Ref{Ref: tc.ref},
			}
			loader, err := v.getLoader(att)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.IsType(tc.expected, loader)
		})
	}
}

func (s *testSuite) TestGetInputArguments() {
	cases := []struct {
		name     string
		inputs   map[string]string
		expected map[string]any
	}{
		{
			name:     "string input",
			inputs:   map[string]string{"foo": "bar"},
			expected: map[string]any{"foo": "bar"},
		},
		{
			name:     "csv input",
			inputs:   map[string]string{"foo": "bar1,bar2,bar3"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}},
		},
		{
			name:     "csv input with empty slots",
			inputs:   map[string]string{"foo": ",bar1,,,bar2,bar3,,"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}},
		},
		{
			name:     "csv input with line feeds",
			inputs:   map[string]string{"foo": "\nbar1,,,bar2,bar3,,"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}},
		},
		{
			name:     "multiline input",
			inputs:   map[string]string{"foo": "\nbar1\nbar2\nbar3\n"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}},
		},
		{
			name:     "multiline input with empty lines",
			inputs:   map[string]string{"foo": "\n\n\nbar1\nbar2\n\nbar3\n"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}},
		},
		{
			name:     "no input",
			inputs:   nil,
			expected: map[string]any{},
		},
		{
			name:     "multiple values",
			inputs:   map[string]string{"foo": "bar1,bar2,bar3", "bar": "baz", "foos": "bar1\nbar2\nbar3\n"},
			expected: map[string]any{"foo": []string{"bar1", "bar2", "bar3"}, "bar": "baz", "foos": []string{"bar1", "bar2", "bar3"}},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			actual := getInputArguments(tc.inputs)
			s.Equal(tc.expected, actual)
		})
	}
}

func (s *testSuite) TestComputePolicyArguments() {
	cases := []struct {
		name      string
		inputs    []*v12.PolicyInput
		args      map[string]string
		bindings  map[string]string
		expected  map[string]string
		expectErr bool
		errMsg    string
	}{
		{
			name:     "all args passed when no inputs present",
			inputs:   nil,
			args:     map[string]string{"arg1": "value1", "arg2": "value2"},
			expected: map[string]string{"arg1": "value1", "arg2": "value2"},
		},
		{
			name: "required inputs",
			inputs: []*v12.PolicyInput{{
				Name:     "arg1",
				Required: true,
			}},
			args:      map[string]string{"arg2": "value2"},
			expectErr: true,
			errMsg:    "missing required input \"arg1\"",
		},
		{
			name: "default values are set",
			inputs: []*v12.PolicyInput{{
				Name:    "arg1",
				Default: "value1",
			}, {
				Name:     "arg2",
				Required: true,
			}},
			args:     map[string]string{"arg2": "value2"},
			expected: map[string]string{"arg1": "value1", "arg2": "value2"},
		},
		{
			name: "unexpected arguments are ignored",
			inputs: []*v12.PolicyInput{{
				Name:    "arg1",
				Default: "value1",
			}, {
				Name: "arg2",
			}},
			args:     map[string]string{"arg3": "value3"},
			expected: map[string]string{"arg1": "value1"},
		},
		{
			name: "expected arguments with values are respected",
			inputs: []*v12.PolicyInput{{
				Name:    "arg1",
				Default: "value1",
			}, {
				Name: "arg2",
			}},
			args:     map[string]string{"arg1": "value1", "arg2": "value2"},
			expected: map[string]string{"arg1": "value1", "arg2": "value2"},
		},
		{
			name: "simple bindings",
			inputs: []*v12.PolicyInput{{
				Name: "arg1",
			}},
			args:     map[string]string{"arg1": "Hello {{ .inputs.foo }}"},
			bindings: map[string]string{"foo": "world"},
			expected: map[string]string{"arg1": "Hello world"},
		},
		{
			name: "multiple bindings",
			inputs: []*v12.PolicyInput{{
				Name: "arg1",
			}, {
				Name: "arg2",
			}},
			args:     map[string]string{"arg1": "Hello {{ .inputs.foo }} {{ .inputs.bar }}", "arg2": "Bye {{ .inputs.bar }}"},
			bindings: map[string]string{"foo": "world", "bar": "template"},
			expected: map[string]string{"arg1": "Hello world template", "arg2": "Bye template"},
		},
		{
			name: "no variable found in bindings, renders zero value",
			inputs: []*v12.PolicyInput{{
				Name: "arg1",
			}},
			args:     map[string]string{"arg1": "Hello {{ .inputs.foo }}"},
			bindings: map[string]string{"bar": "world"},
			expected: map[string]string{"arg1": "Hello "},
		},
		{
			name: "no interpolation needed",
			inputs: []*v12.PolicyInput{{
				Name: "arg1",
			}},
			args:     map[string]string{"arg1": "Hello world"},
			bindings: map[string]string{"foo": "bar"},
			expected: map[string]string{"arg1": "Hello world"},
		},
		{
			name: "required and default is illegal",
			inputs: []*v12.PolicyInput{{
				Name:     "arg1",
				Required: true,
				Default:  "foo",
			}},
			args:      map[string]string{"arg1": "Hello world"},
			expectErr: true,
			errMsg:    "input arg1 can not be required and have a default at the same time",
		},
		{
			name: "inputs prefix without dot",
			inputs: []*v12.PolicyInput{{
				Name: "arg1",
			}, {
				Name: "arg2",
			}},
			args:     map[string]string{"arg1": "Hello {{inputs.foo }} {{   inputs.bar }}", "arg2": "Bye {{ inputs.bar}}"},
			bindings: map[string]string{"foo": "world", "bar": "template"},
			expected: map[string]string{"arg1": "Hello world template", "arg2": "Bye template"},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			computed, err := ComputeArguments(tc.inputs, tc.args, tc.bindings, &s.logger)
			if tc.expectErr {
				s.Error(err)
				s.Contains(err.Error(), tc.errMsg)
				return
			}
			s.NoError(err)
			s.Equal(tc.expected, computed)
		})
	}
}

func (s *testSuite) TestNewResultFormat() {
	cases := []struct {
		name             string
		policy           string
		material         string
		expectErr        bool
		expectViolations int
		expectSkipped    bool
		expectReasons    []string
	}{
		{
			name:             "result.violations",
			policy:           "file://testdata/policy_result_format.yaml",
			material:         "{\"specVersion\": \"1.4\"}",
			expectViolations: 1,
		},
		{
			name:          "skip",
			policy:        "file://testdata/policy_result_format.yaml",
			material:      "{\"invalid\": \"1.4\"}",
			expectSkipped: true,
			expectReasons: []string{"invalid input"},
		},
		{
			name:          "skip multiple",
			policy:        "file://testdata/policy_result_skipped.yaml",
			material:      "{}",
			expectSkipped: true,
			expectReasons: []string{"this one is skipped", "this is also skipped"},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			schema := &v12.CraftingSchema{
				Materials: []*v12.CraftingSchema_Material{
					{
						Name: "sbom",
						Type: v12.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
					},
				},
				Policies: &v12.Policies{
					Materials: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: tc.policy},
						},
					},
					Attestation: nil,
				},
			}
			material := &v1.Attestation_Material{
				M: &v1.Attestation_Material_Artifact_{Artifact: &v1.Attestation_Material_Artifact{
					Content: []byte(tc.material),
				}},
				MaterialType: v12.CraftingSchema_Material_SBOM_CYCLONEDX_JSON,
				InlineCas:    true,
			}

			verifier := NewPolicyVerifier(schema, nil, &s.logger)
			res, err := verifier.VerifyMaterial(context.TODO(), material, "")

			if tc.expectErr {
				s.Error(err)
				return
			}

			s.Require().NoError(err)
			s.Len(res, 1)
			s.Len(res[0].Violations, tc.expectViolations)
			s.Equal(tc.expectSkipped, res[0].Skipped)
			if len(res[0].SkipReasons) > 0 {
				s.Equal(res[0].SkipReasons, tc.expectReasons)
			}
		})
	}
}

func (s *testSuite) TestContainerMaterial() {
	cases := []struct {
		name          string
		policy        string
		tag           string
		expectErr     bool
		expectSkipped bool
		expectReasons []string
	}{
		{
			name: "containers",
			// This policy injects the container tag in the `skip_reason` field
			policy:        "file://testdata/container_policy.yaml",
			tag:           "latest",
			expectSkipped: true,
			expectReasons: []string{"the tag is 'latest'"},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			schema := &v12.CraftingSchema{
				Materials: []*v12.CraftingSchema_Material{
					{
						Name: "the-container",
						Type: v12.CraftingSchema_Material_CONTAINER_IMAGE,
					},
				},
				Policies: &v12.Policies{
					Materials: []*v12.PolicyAttachment{
						{
							Policy: &v12.PolicyAttachment_Ref{Ref: tc.policy},
						},
					},
					Attestation: nil,
				},
			}
			material := &v1.Attestation_Material{
				Id: "material-1729779925030105000",
				M: &v1.Attestation_Material_ContainerImage_{ContainerImage: &v1.Attestation_Material_ContainerImage{
					Tag:               tc.tag,
					SignatureProvider: "cosign",
				}},
				MaterialType: v12.CraftingSchema_Material_CONTAINER_IMAGE,
			}

			verifier := NewPolicyVerifier(schema, nil, &s.logger)
			res, err := verifier.VerifyMaterial(context.TODO(), material, "")

			if tc.expectErr {
				s.Error(err)
				return
			}

			s.Require().NoError(err)
			s.Len(res, 1)
			s.Equal(tc.expectSkipped, res[0].Skipped)
			if len(res[0].SkipReasons) > 0 {
				s.Equal(res[0].SkipReasons, tc.expectReasons)
			}
		})
	}
}

type testSuite struct {
	suite.Suite

	logger zerolog.Logger
}

func (s *testSuite) SetupTest() {
	s.logger = zerolog.Nop()
}

func TestPolicyVerifier(t *testing.T) {
	suite.Run(t, new(testSuite))
}

func loadStatement(file string, s *suite.Suite) *intoto.Statement {
	stContent, err := os.ReadFile(file)
	s.Require().NoError(err)
	var statement intoto.Statement
	err = protojson.Unmarshal(stContent, &statement)
	s.Require().NoError(err)

	return &statement
}
