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
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
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

func (m *SLSACommonProvenanceMaterial) String() (res string) {
	// we just care about the first one
	for alg, h := range m.Digest {
		res = fmt.Sprintf("%s@%s:%s", m.URI, alg, h)
	}

	return
}

func (m *ProvenanceM) String() string {
	if m.SLSA != nil {
		return m.SLSA.String()
	}

	return m.StringVal
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
		nMaterial := mdef.NormalizedOutput()

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material := &ProvenanceM{}
		if artifactType == schemaapi.CraftingSchema_Material_STRING {
			material.StringVal = nMaterial.Value
		} else if nMaterial.Digest != "" {
			parts := strings.Split(nMaterial.Digest, ":")
			material.SLSA = &SLSACommonProvenanceMaterial{
				&slsacommon.ProvenanceMaterial{
					URI: nMaterial.Value,
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

// Extract the Chainloop attestation predicate from an encoded DSSE envelope
func ExtractPredicate(envelope *dsse.Envelope) (*ProvenancePredicateVersions, error) {
	decodedPayload, err := envelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	// 1 - Extract the in-toto statement
	statement := &in_toto.Statement{}
	if err := json.Unmarshal(decodedPayload, statement); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	// 2 - Extract the Chainloop predicate from the in-toto statement
	switch statement.PredicateType {
	case PredicateTypeV01:
		var predicate *ProvenancePredicateV01
		if err = extractPredicate(statement, &predicate); err != nil {
			return nil, fmt.Errorf("extracting predicate: %w", err)
		}

		return &ProvenancePredicateVersions{V01: predicate}, nil
	default:
		return nil, fmt.Errorf("unsupported predicate type: %s", statement.PredicateType)
	}
}
