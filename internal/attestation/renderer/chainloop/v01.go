//
// Copyright 2023 The Chainloop Authors.
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

package chainloop

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/in-toto/in-toto-golang/in_toto"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const PredicateTypeV01 = "chainloop.dev/attestation/v0.1"

type ProvenancePredicateV01 struct {
	*ProvenancePredicateCommon
	Materials []*ProvenanceMaterial `json:"materials,omitempty"`
}

type ProvenanceMaterial struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Material *ProvenanceM `json:"material"`
}

type SLSACommonProvenanceMaterial struct {
	*slsacommon.ProvenanceMaterial
}

type ProvenanceM struct {
	SLSA      *SLSACommonProvenanceMaterial `json:"slsa,omitempty"`
	StringVal string                        `json:"stringVal,omitempty"`
}

type RendererV01 struct {
	*RendererCommon
}

func NewChainloopRendererV01(att *v1.Attestation, builderVersion, builderDigest string) *RendererV01 {
	return &RendererV01{&RendererCommon{
		PredicateTypeV01, att, &builderInfo{builderVersion, builderDigest}},
	}
}

func (r *RendererV01) Predicate() (interface{}, error) {
	return ProvenancePredicateV01{
		ProvenancePredicateCommon: predicateCommon(r.builder, r.att),
		Materials:                 outputChainloopMaterials(r.att, false),
	}, nil
}

func (r *RendererV01) Header() (*in_toto.StatementHeader, error) {
	raw, err := json.Marshal(r.att)
	if err != nil {
		return nil, err
	}

	// We might don't want this and just force the existence of one material with output = true
	subjects := []in_toto.Subject{
		{
			Name: fmt.Sprintf("chainloop.dev/workflow/%s", r.att.GetWorkflow().Name),
			Digest: map[string]string{
				"sha256": fmt.Sprintf("%x", sha256.Sum256(raw)),
			},
		},
	}

	for _, m := range outputChainloopMaterials(r.att, true) {
		if slsaMaterial := m.Material.SLSA.ProvenanceMaterial; slsaMaterial != nil {
			subjects = append(subjects, in_toto.Subject{
				Name:   slsaMaterial.URI,
				Digest: slsaMaterial.Digest,
			})
		}
	}

	return &in_toto.StatementHeader{
		Type:          in_toto.StatementInTotoV01,
		PredicateType: r.predicateType,
		Subject:       subjects,
	}, nil
}

func outputChainloopMaterials(att *v1.Attestation, onlyOutput bool) []*ProvenanceMaterial {
	// Sort material keys to stabilize output
	keys := make([]string, 0, len(att.GetMaterials()))
	for k := range att.GetMaterials() {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := []*ProvenanceMaterial{}
	materials := att.GetMaterials()
	for _, mdefName := range keys {
		mdef := materials[mdefName]

		artifactType := mdef.MaterialType
		nMaterial, err := mdef.NormalizedOutput()
		if err != nil {
			continue
		}

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material := &ProvenanceM{}
		if artifactType == schemaapi.CraftingSchema_Material_STRING {
			material.StringVal = string(nMaterial.Content)
		} else if nMaterial.Digest != "" {
			parts := strings.Split(nMaterial.Digest, ":")
			material.SLSA = &SLSACommonProvenanceMaterial{
				&slsacommon.ProvenanceMaterial{
					URI: nMaterial.Name,
					Digest: map[string]string{
						parts[0]: parts[1],
					},
				},
			}
		}

		res = append(res, &ProvenanceMaterial{
			Material: material,
			Name:     mdefName,
			Type:     artifactType.String(),
		})
	}

	return res
}

// Implement NormalizablePredicate
// Override
func (p *ProvenancePredicateV01) GetMaterials() []*NormalizedMaterial {
	res := make([]*NormalizedMaterial, 0, len(p.Materials))
	for _, m := range p.Materials {
		nm := &NormalizedMaterial{
			Name: m.Name,
			Type: m.Type,
		}

		if m.Material.StringVal != "" {
			nm.Value = m.Material.StringVal
		} else if m.Material.SLSA != nil {
			nm.Value = m.Material.SLSA.URI
			nm.Hash = &crv1.Hash{Algorithm: "sha256", Hex: m.Material.SLSA.Digest["sha256"]}
		}

		res = append(res, nm)
	}

	return res
}
