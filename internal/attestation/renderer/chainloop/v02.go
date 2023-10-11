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
	normalizedMaterials, err := outputMaterials(r.att, false)
	if err != nil {
		return nil, fmt.Errorf("error normalizing materials: %w", err)
	}

	return ProvenancePredicateV02{
		ProvenancePredicateCommon: predicateCommon(r.builder, r.att),
		Materials:                 normalizedMaterials,
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
			Name: prefixed(fmt.Sprintf("workflow.%s", r.att.GetWorkflow().Name)),
			Digest: map[string]string{
				"sha256": fmt.Sprintf("%x", sha256.Sum256(raw)),
			},
		},
	}

	if r.att.GetSha1Commit() != "" {
		subjects = append(subjects, in_toto.Subject{
			Name:   subjectGitHead,
			Digest: map[string]string{"sha1": r.att.GetSha1Commit()},
		})
	}

	normalizedMaterials, err := outputMaterials(r.att, true)
	if err != nil {
		return nil, fmt.Errorf("error normalizing materials: %w", err)
	}

	for _, m := range normalizedMaterials {
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

func outputMaterials(att *v1.Attestation, onlyOutput bool) ([]*slsa_v1.ResourceDescriptor, error) {
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
		nMaterial, err := mdef.NormalizedOutput()
		if err != nil {
			return nil, fmt.Errorf("error normalizing material: %w", err)
		}

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material := &slsa_v1.ResourceDescriptor{}
		if artifactType == schemaapi.CraftingSchema_Material_STRING {
			material.Content = nMaterial.Content
		}

		if digest := nMaterial.Digest; digest != "" {
			parts := strings.Split(digest, ":")
			material.Digest = map[string]string{
				parts[0]: parts[1],
			}
			material.Name = nMaterial.Name
			material.Content = nMaterial.Content
		}

		// Required, built-in annotations
		material.Annotations = map[string]interface{}{
			annotationMaterialType: artifactType.String(),
			annotationMaterialName: mdefName,
		}

		// Custom annotations, it does not override the built-in ones
		for k, v := range mdef.Annotations {
			_, ok := material.Annotations[k]
			if !ok {
				material.Annotations[k] = v
			}
		}

		if mdef.UploadedToCas {
			material.Annotations[annotationMaterialCAS] = true
		} else if mdef.InlineCas {
			material.Annotations[annotationMaterialInlineCAS] = true
		}

		res = append(res, material)
	}

	return res, nil
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

	// Set custom annotations
	m.Annotations = make(map[string]string)
	for k, v := range material.Annotations {
		// if the annotation key doesn't start with chainloop.
		// we set it as a custom annotation
		if strings.HasPrefix(k, rendererPrefix) {
			continue
		}

		m.Annotations[k] = v.(string)
	}

	mType, ok := material.Annotations[annotationMaterialType]
	if !ok {
		return nil, fmt.Errorf("material type not found")
	}

	// Set the type
	m.Type = mType.(string)

	mName, ok := material.Annotations[annotationMaterialName]
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
	// material.Name in a container image is the path to the image
	// in an artifact type or derivative means the name of the file
	if material.Name == "" {
		return nil, fmt.Errorf("material name not found")
	}

	// In the case of container images for example the value is in the name field
	m.Value = material.Name

	if v, ok := material.Annotations[annotationMaterialCAS]; ok && v.(bool) {
		m.UploadedToCAS = true
	}

	if v, ok := material.Annotations[annotationMaterialInlineCAS]; ok && v.(bool) {
		m.EmbeddedInline = true
	}

	// In the case of an artifact type or derivative the filename is set and the inline content if any
	if m.EmbeddedInline || m.UploadedToCAS {
		m.Filename = material.Name
		m.Value = ""
	}

	if m.EmbeddedInline {
		m.Value = string(material.Content)
	}

	return m, nil
}
