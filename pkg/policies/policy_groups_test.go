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
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type groupsTestSuite struct {
	suite.Suite

	logger zerolog.Logger
}

func (s *groupsTestSuite) SetupTest() {
	s.logger = zerolog.Nop()
}

func TestPolicyGroups(t *testing.T) {
	suite.Run(t, new(groupsTestSuite))
}

func (s *groupsTestSuite) TestLoadGroupSpec() {
	var cases = []struct {
		name             string
		attachment       *v1.PolicyGroupAttachment
		wantErr          bool
		expectedName     string
		expectedDesc     string
		expectedCategory string
	}{
		{
			name:       "missing ref",
			attachment: &v1.PolicyGroupAttachment{},
			wantErr:    true,
		},
		{
			name: "by file ref",
			attachment: &v1.PolicyGroupAttachment{
				Ref: "file://testdata/policy_group.yaml",
			},
			expectedName: "sbom-quality",
		},
		{
			name: "with wrong digest",
			attachment: &v1.PolicyGroupAttachment{
				Ref: "file://testdata/policy_group.yaml@sha256:24c4bd4f56b470d7436ed0c5a340483fff9ad058033f94b164f5efc59aba5136",
			},
			expectedName: "sbom-quality",
			wantErr:      true,
		},
		{
			name: "with correct digest",
			attachment: &v1.PolicyGroupAttachment{
				Ref: "file://testdata/policy_group.yaml@sha256:1c21e5bb5323934a6c4827e760fa3fe20086470f83b2c99a2cdb75fc0b066035",
			},
			expectedName: "sbom-quality",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			verifier := NewPolicyGroupVerifier(nil, nil, &s.logger)
			group, _, err := verifier.loadPolicyGroup(context.TODO(), tc.attachment)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.expectedName, group.GetMetadata().GetName())
			if tc.expectedDesc != "" {
				s.Equal(tc.expectedDesc, group.Metadata.Description)
			}

			if tc.expectedCategory != "" {
				s.Equal(tc.expectedCategory, group.Metadata.Annotations["category"])
			}
		})
	}
}

func (s *groupsTestSuite) TestRequiredPoliciesForMaterial() {
	cases := []struct {
		name         string
		schemaRef    string
		materialType v1.CraftingSchema_Material_MaterialType
		expected     int
	}{
		{
			name:         "no match",
			schemaRef:    "file://testdata/policy_group.yaml",
			materialType: v1.CraftingSchema_Material_GITLAB_SECURITY_REPORT,
			expected:     0,
		},
		{
			name:         "match",
			schemaRef:    "file://testdata/policy_group.yaml",
			materialType: v1.CraftingSchema_Material_SBOM_SPDX_JSON,
			expected:     1,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			schema := &v1.CraftingSchema{PolicyGroups: []*v1.PolicyGroupAttachment{{Ref: tc.schemaRef}}}

			material := &api.Attestation_Material{
				MaterialType: tc.materialType,
			}

			v := NewPolicyGroupVerifier(schema, nil, &s.logger)
			attsMap, err := v.requiredPolicyGroupsForMaterial(context.TODO(), material)
			s.Require().NoError(err)
			s.Len(attsMap["sbom-quality"], tc.expected)
		})
	}
}

func (s *groupsTestSuite) TestGroupLoader() {
	cases := []struct {
		name     string
		ref      string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "file ref",
			ref:      "file://local-policy.yaml",
			expected: &FileGroupLoader{},
		},
		{
			name:     "http ref",
			ref:      "https://myhost/policy.yaml",
			expected: &HTTPSGroupLoader{},
		},
		{
			name:    "invalid ref",
			ref:     "env://environmentvar",
			wantErr: true,
		},
		{
			name:    "empty ref",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			v := NewPolicyGroupVerifier(nil, nil, &s.logger)
			att := &v1.PolicyGroupAttachment{Ref: tc.ref}
			loader, err := v.getGroupLoader(att)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.IsType(tc.expected, loader)
		})
	}
}

func (s *groupsTestSuite) TestVerifyAttestations() {
	cases := []struct {
		name       string
		schema     *v1.CraftingSchema
		statement  string
		npolicies  int
		violations int
		wantErr    error
		groupName  string
	}{
		{
			name: "test attestation with violations",
			schema: &v1.CraftingSchema{
				PolicyGroups: []*v1.PolicyGroupAttachment{
					{
						Ref: "file://testdata/policy_group.yaml",
					},
				},
			},
			statement:  "testdata/statement.json",
			npolicies:  1,
			violations: 1,
			groupName:  "sbom-quality",
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			v := NewPolicyGroupVerifier(tc.schema, nil, &s.logger)
			statement := loadStatement(tc.statement, &s.Suite)
			res, err := v.VerifyStatement(context.TODO(), statement)
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
					s.Equal(tc.groupName, pol.GroupName)
				}
				s.Equal(tc.violations, violations)
			}
		})
	}
}
