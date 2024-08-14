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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/wrong_policy.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/missing_rego.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow_embedded.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow_embedded.yaml"}},
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
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/workflow_embedded.yaml"}},
						{Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/materials.yaml"}},
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
							Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/with_arguments.yaml"},
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
							Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/with_arguments.yaml"},
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
							Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/with_arguments.yaml"},
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
							Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/with_arguments.yaml"},
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
							Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/with_arguments.yaml"},
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
			stContent, err := os.ReadFile(tc.statement)
			s.Require().NoError(err)
			var statement intoto.Statement
			err = protojson.Unmarshal(stContent, &statement)
			s.Require().NoError(err)

			res, err := verifier.VerifyStatement(context.TODO(), &statement)
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

func (s *testSuite) TestMaterialSelectionCriteria() {
	attNoFilterPolicyTyped := &v12.PolicyAttachment{
		Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/sbom_syft.yaml"},
	}
	attFilteredPolicyTyped := &v12.PolicyAttachment{
		Policy:   &v12.PolicyAttachment_Ref{Ref: "testdata/sbom_syft.yaml"},
		Selector: &v12.PolicyAttachment_MaterialSelector{Name: "sbom"},
	}
	attFilteredPolicyNotTyped := &v12.PolicyAttachment{
		Policy:   &v12.PolicyAttachment_Ref{Ref: "testdata/sbom_syft_not_typed.yaml"},
		Selector: &v12.PolicyAttachment_MaterialSelector{Name: "custom-material"},
	}
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
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{
						Id: "sbom",
					},
				},
				MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON},
			result: 1,
		},
		{
			name:     "attachment with filter, policy with type, unmatched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyTyped},
			material: &v1.Attestation_Material{
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{
						Id: "not-the-sbom-you-expect",
					},
				},
				MaterialType: v12.CraftingSchema_Material_SBOM_SPDX_JSON},
			result: 0,
		},
		{
			name:     "attachment with no filter, policy without type, matched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyNotTyped},
			material: &v1.Attestation_Material{
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{
						Id: "custom-material",
					},
				},
				MaterialType: v12.CraftingSchema_Material_ATTESTATION},
			result: 1,
		},
		{
			name:     "attachment with no filter, policy without type, unmatched material",
			policies: []*v12.PolicyAttachment{attFilteredPolicyNotTyped},
			material: &v1.Attestation_Material{
				M: &v1.Attestation_Material_Artifact_{
					Artifact: &v1.Attestation_Material_Artifact{
						Id: "not-the-material-you-expect",
					},
				},
				MaterialType: v12.CraftingSchema_Material_ATTESTATION},
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
					Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/sbom_syft.yaml"},
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
					Policy: &v12.PolicyAttachment_Ref{Ref: "testdata/sbom_syft.yaml"},
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
	}{
		{
			name:       "missing policy",
			attachment: &v12.PolicyAttachment{},
			wantErr:    true,
		},
		{
			name: "by ref",
			attachment: &v12.PolicyAttachment{
				Policy: &v12.PolicyAttachment_Ref{
					Ref: "testdata/sbom_syft.yaml",
				},
			},
			expectedName:     "made-with-syft",
			expectedDesc:     "This policy checks that the SPDX SBOM was created with syft",
			expectedCategory: "SBOM",
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
		},
	}

	verifier := NewPolicyVerifier(nil, nil, &s.logger)
	for _, tc := range cases {
		s.Run(tc.name, func() {
			p, err := verifier.loadPolicySpec(context.TODO(), tc.attachment)
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
			name:     "local ref",
			ref:      "local-policy.yaml",
			expected: &BlobLoader{},
		},
		{
			name:     "http ref",
			ref:      "https://myhost/policy.yaml",
			expected: &BlobLoader{},
		},
		{
			name:     "env ref",
			ref:      "env://environmentvar",
			expected: &BlobLoader{},
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

func (s *testSuite) TestInputArguments() {
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
