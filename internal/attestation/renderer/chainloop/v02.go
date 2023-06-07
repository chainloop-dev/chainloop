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
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/in-toto/in-toto-golang/in_toto"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
)

// Replace custom material type with https://github.com/in-toto/attestation/blob/main/spec/v1.0/resource_descriptor.md
const PredicateTypeV02 = "chainloop.dev/attestation/v0.2"

type ProvenancePredicateV02 struct {
	*ProvenancePredicateCommon
	Materials []*slsa_v1.ResourceDescriptor `json:"materials,omitempty"`
}

type RendererV02 struct {
	*RendererCommon
}

func NewChainloopRendererV02(att *v1.Attestation, builderVersion, builderDigest string) *RendererV02 {
	return &RendererV02{&RendererCommon{
		PredicateTypeV02, att, &builderInfo{builderVersion, builderDigest}},
	}
}

func (r *RendererV02) Predicate() (interface{}, error) {
	return ProvenancePredicateV02{
		ProvenancePredicateCommon: predicateCommon(r.builder, r.att),
		Materials:                 outputMaterials(r.att, false),
	}, nil
}

func (r *RendererV02) Header() (*in_toto.StatementHeader, error) {
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

	for _, m := range outputMaterials(r.att, true) {
		if m.Digest != nil {
			subjects = append(subjects, in_toto.Subject{
				Name:   m.Name,
				Digest: m.Digest,
			})
		}
	}

	return &in_toto.StatementHeader{
		Type:          in_toto.StatementInTotoV01,
		PredicateType: r.predicateType,
		Subject:       subjects,
	}, nil
}

const AnnotationMaterialType = "chainloop.material.type"
const AnnotationMaterialName = "chainloop.material.name"
const AnnotationMaterialCAS = "chainloop.material.cas"

func outputMaterials(att *v1.Attestation, onlyOutput bool) []*slsa_v1.ResourceDescriptor {
	// Sort material keys to stabilize output
	keys := make([]string, 0, len(att.GetMaterials()))
	for k := range att.GetMaterials() {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := []*slsa_v1.ResourceDescriptor{}
	materials := att.GetMaterials()
	for _, mdefName := range keys {
		mdef := materials[mdefName]

		artifactType := mdef.MaterialType
		nMaterial := mdef.NormalizedOutput()

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material := &slsa_v1.ResourceDescriptor{}
		if artifactType == schemaapi.CraftingSchema_Material_STRING {
			material.Content = []byte(nMaterial.Value)
		}

		if digest := nMaterial.Digest; digest != "" {
			parts := strings.Split(digest, ":")
			material.Digest = map[string]string{
				parts[0]: parts[1],
			}
			material.Name = nMaterial.Value
		}

		material.Annotations = map[string]interface{}{
			AnnotationMaterialType: artifactType.String(),
			AnnotationMaterialName: mdefName,
		}

		if mdef.UploadedToCas {
			material.Annotations[AnnotationMaterialCAS] = true
		}

		res = append(res, material)
	}

	return res
}

// Implement NormalizablePredicate interface
func (p *ProvenancePredicateV02) GetMaterials() []*NormalizedMaterial {
	res := make([]*NormalizedMaterial, 0, len(p.Materials))
	for _, material := range p.Materials {
		m, err := normalizeMaterial(material)
		if err != nil {
			continue
		}

		res = append(res, m)
	}

	return res
}

// Translate a ResourceDescriptor to a NormalizedMaterial
func normalizeMaterial(material *slsa_v1.ResourceDescriptor) (*NormalizedMaterial, error) {
	m := &NormalizedMaterial{}

	mType, ok := material.Annotations[AnnotationMaterialType]
	if !ok {
		return nil, fmt.Errorf("material type not found")
	}

	// Set the type
	m.Type = mType.(string)

	mName, ok := material.Annotations[AnnotationMaterialName]
	if !ok {
		return nil, fmt.Errorf("material name not found")
	}

	// Set the Material Name
	m.Name = mName.(string)

	// Set the Value
	// If we have a string material, we just set the value
	if m.Type == schemaapi.CraftingSchema_Material_STRING.String() {
		if material.Content == nil {
			return nil, fmt.Errorf("material content not found")
		}

		m.Value = string(material.Content)

		return m, nil
	}

	// for the rest of the materials we use both the name and the digest
	d, ok := material.Digest["sha256"]
	if !ok {
		return nil, fmt.Errorf("material digest not found")
	}

	m.Hash = &crv1.Hash{Algorithm: "sha256", Hex: d}
	if material.Name == "" {
		return nil, fmt.Errorf("material name not found")
	}

	if v, ok := material.Annotations[AnnotationMaterialCAS]; ok && v.(bool) {
		m.DownloadableFromCAS = true
	}

	m.Value = material.Name

	return m, nil
}
