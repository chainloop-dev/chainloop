//
// Copyright 2024-2026 The Chainloop Authors.
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
	"testing"

	craftingpb "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestPopulateContractMaterials(t *testing.T) {
	tests := []struct {
		name           string
		craftingState  *v1.CraftingState
		attRes         *AttestationStatusResult
		totalMaterials int
		wantErr        bool
	}{
		{
			name: "empty attestation result",
		},
		{
			name:           "materials on contract",
			totalMaterials: 1,
			craftingState: &v1.CraftingState{
				Schema: &v1.CraftingState_InputSchema{
					InputSchema: &craftingpb.CraftingSchema{
						SchemaVersion: "v1",
						Materials: []*craftingpb.CraftingSchema_Material{
							{
								Type:   craftingpb.CraftingSchema_Material_CSAF_VEX,
								Name:   "vex-file",
								Output: true,
							},
						},
					},
				},
			},
		},
		{
			name:           "materials in contract and outside contract",
			totalMaterials: 2,
			craftingState: &v1.CraftingState{
				Schema: &v1.CraftingState_InputSchema{
					InputSchema: &craftingpb.CraftingSchema{
						SchemaVersion: "v1",
						Materials: []*craftingpb.CraftingSchema_Material{
							{
								Type:   craftingpb.CraftingSchema_Material_CSAF_VEX,
								Name:   "vex-file",
								Output: true,
							},
						},
					},
				},
				Attestation: &v1.Attestation{
					Materials: map[string]*v1.Attestation_Material{
						"vex-file": {
							Id: "random",
							M: &v1.Attestation_Material_Artifact_{
								Artifact: &v1.Attestation_Material_Artifact{
									Name:    "vex-file",
									Digest:  "random-digest",
									Content: []byte("random-content"),
								},
							},
							MaterialType:  craftingpb.CraftingSchema_Material_CSAF_VEX,
							UploadedToCas: true,
						},
						"other-file": {
							Id: "random",
							M: &v1.Attestation_Material_Artifact_{
								Artifact: &v1.Attestation_Material_Artifact{
									Name:    "other-file",
									Digest:  "random-digest-2",
									Content: []byte("random-content-2"),
								},
							},
							MaterialType:  craftingpb.CraftingSchema_Material_CSAF_SECURITY_ADVISORY,
							UploadedToCas: true,
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.attRes = &AttestationStatusResult{}

			err := populateMaterials(tc.craftingState, tc.attRes)
			assert.NoError(t, err)

			for _, m := range tc.attRes.Materials {
				assert.NotNil(t, m)
				assert.NotEmpty(t, m.Name)
				assert.NotEmpty(t, m.Type)
			}
			assert.Equal(t, tc.totalMaterials, len(tc.attRes.Materials))
		})
	}
}

func TestPopulateContractMaterialsGroups(t *testing.T) {
	state := &v1.CraftingState{
		Schema: &v1.CraftingState_InputSchema{
			InputSchema: &craftingpb.CraftingSchema{
				SchemaVersion: "v1",
				Materials: []*craftingpb.CraftingSchema_Material{
					{Type: craftingpb.CraftingSchema_Material_ARTIFACT, Name: "required-one"},
					{Type: craftingpb.CraftingSchema_Material_ARTIFACT, Name: "optional-one", Optional: true},
					{Type: craftingpb.CraftingSchema_Material_SBOM_CYCLONEDX_JSON, Name: "sbom-cyclonedx", Group: "sbom"},
					{Type: craftingpb.CraftingSchema_Material_SBOM_SPDX_JSON, Name: "sbom-spdx", Group: "sbom"},
				},
			},
		},
	}

	res := &AttestationStatusResult{}
	err := populateMaterials(state, res)
	assert.NoError(t, err)

	byName := make(map[string]AttestationStatusMaterial, len(res.Materials))
	for _, m := range res.Materials {
		byName[m.Name] = m
	}

	// Ungrouped required material: Required, no group
	assert.True(t, byName["required-one"].Required)
	assert.Empty(t, byName["required-one"].Group)

	// Ungrouped optional material: not required, no group
	assert.False(t, byName["optional-one"].Required)
	assert.Empty(t, byName["optional-one"].Group)

	// Grouped materials: carry the group and are NOT individually required
	assert.Equal(t, "sbom", byName["sbom-cyclonedx"].Group)
	assert.False(t, byName["sbom-cyclonedx"].Required)
	assert.Equal(t, "sbom", byName["sbom-spdx"].Group)
	assert.False(t, byName["sbom-spdx"].Required)
}

func TestPolicyEnforcementSource(t *testing.T) {
	contractV2 := func(gate *bool) *v1.CraftingState_SchemaV2 {
		return &v1.CraftingState_SchemaV2{SchemaV2: &craftingpb.CraftingSchemaV2{
			Spec: &craftingpb.CraftingSchemaV2Spec{Policies: &craftingpb.Policies{Gate: gate}},
		}}
	}

	testCases := []struct {
		name  string
		state *v1.CraftingState
		want  string
	}{
		{name: "nothing enforces", state: &v1.CraftingState{Attestation: &v1.Attestation{}, Schema: contractV2(nil)}},
		{
			name:  "organization",
			state: &v1.CraftingState{Attestation: &v1.Attestation{BlockOnPolicyViolation: true}, Schema: contractV2(nil)},
			want:  PolicyEnforcementSourceOrganization,
		},
		{
			name:  "contract, also when the organization enforces",
			state: &v1.CraftingState{Attestation: &v1.Attestation{BlockOnPolicyViolation: true}, Schema: contractV2(proto.Bool(true))},
			want:  PolicyEnforcementSourceContract,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policyEnforcementSource(tc.state))
		})
	}
}

func TestIgnoredGateOptOuts(t *testing.T) {
	const (
		refPolicy      = "sbom-banned-licenses"
		embeddedPolicy = "embedded-policy"
		group          = "slsa-checks"
		warningPrefix  = "gate: false has no effect on "
	)

	policies := func(contractGate bool) *craftingpb.Policies {
		return &craftingpb.Policies{
			Gate: proto.Bool(contractGate),
			Materials: []*craftingpb.PolicyAttachment{
				{Policy: &craftingpb.PolicyAttachment_Ref{Ref: refPolicy}, Gate: proto.Bool(false)},
				{Policy: &craftingpb.PolicyAttachment_Ref{Ref: "sbom-present"}},
			},
			Attestation: []*craftingpb.PolicyAttachment{
				{
					Policy: &craftingpb.PolicyAttachment_Embedded{Embedded: &craftingpb.Policy{Metadata: &craftingpb.Metadata{Name: embeddedPolicy}}},
					Gate:   proto.Bool(false),
				},
			},
		}
	}
	groups := []*craftingpb.PolicyGroupAttachment{
		{Ref: group, Gate: proto.Bool(false)},
		{Ref: "sast"},
	}
	schemaV2 := func(contractGate bool) *v1.CraftingState_SchemaV2 {
		return &v1.CraftingState_SchemaV2{SchemaV2: &craftingpb.CraftingSchemaV2{
			Spec: &craftingpb.CraftingSchemaV2Spec{Policies: policies(contractGate), PolicyGroups: groups},
		}}
	}
	ignored := []string{refPolicy, embeddedPolicy, group}

	testCases := []struct {
		name        string
		state       *v1.CraftingState
		want        []string
		wantWarning string
	}{
		{
			name:  "gate false is honored when nothing enforces",
			state: &v1.CraftingState{Attestation: &v1.Attestation{}, Schema: schemaV2(false)},
		},
		{
			name:  "gate false is ignored when the organization enforces, contract-wide opt-out included",
			state: &v1.CraftingState{Attestation: &v1.Attestation{BlockOnPolicyViolation: true}, Schema: schemaV2(false)},
			want:  append([]string{"spec.policies.gate"}, ignored...),
			wantWarning: warningPrefix + "spec.policies.gate, sbom-banned-licenses, embedded-policy, slsa-checks " +
				"because policies are enforced by the organization, and enforcement cannot be relaxed",
		},
		{
			name:  "gate false is ignored when the contract enforces",
			state: &v1.CraftingState{Attestation: &v1.Attestation{BlockOnPolicyViolation: true}, Schema: schemaV2(true)},
			want:  ignored,
			wantWarning: warningPrefix + "sbom-banned-licenses, embedded-policy, slsa-checks " +
				"because policies are enforced by the contract, and enforcement cannot be relaxed",
		},
		{
			name: "v1 contracts name the top-level contract-wide opt-out",
			state: &v1.CraftingState{
				Attestation: &v1.Attestation{BlockOnPolicyViolation: true},
				Schema: &v1.CraftingState_InputSchema{InputSchema: &craftingpb.CraftingSchema{
					SchemaVersion: "v1", Policies: &craftingpb.Policies{Gate: proto.Bool(false)},
				}},
			},
			want: []string{"policies.gate"},
			wantWarning: warningPrefix + "policies.gate " +
				"because policies are enforced by the organization, and enforcement cannot be relaxed",
		},
		{
			name:  "a contract without policies does not panic",
			state: &v1.CraftingState{Attestation: &v1.Attestation{BlockOnPolicyViolation: true}, Schema: &v1.CraftingState_SchemaV2{SchemaV2: &craftingpb.CraftingSchemaV2{}}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ignoredGateOptOuts(tc.state))
			assert.Equal(t, tc.wantWarning, IgnoredGateOptOutsWarning(tc.state))
		})
	}
}
