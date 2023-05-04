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

package renderer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const ChainloopPredicateTypeV1 = "chainloop.dev/attestation/v0.1"

// TODO: Figure out a more appropriate meaning
const chainloopBuildType = "chainloop.dev/workflowrun/v0.1"

const builderIDFmt = "chainloop.dev/cli/%s@%s"

type ChainloopProvenancePredicateV1 struct {
	Metadata   *ChainloopMetadata             `json:"metadata"`
	Materials  []*ChainloopProvenanceMaterial `json:"materials,omitempty"`
	Builder    *slsacommon.ProvenanceBuilder  `json:"builder"`
	BuildType  string                         `json:"buildType"`
	Env        map[string]string              `json:"env,omitempty"`
	RunnerType string                         `json:"runnerType"`
	RunnerURL  string                         `json:"runnerURL,omitempty"`
}

type ChainloopProvenanceMaterial struct {
	Name     string                `json:"name"`
	Type     string                `json:"type"`
	Material *ChainloopProvenanceM `json:"material"`
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

func (m *ChainloopProvenanceM) String() string {
	if m.SLSA != nil {
		return m.SLSA.String()
	}

	return m.StringVal
}

type ChainloopProvenanceM struct {
	SLSA      *SLSACommonProvenanceMaterial `json:"slsa,omitempty"`
	StringVal string                        `json:"stringVal,omitempty"`
}

type ChainloopMetadata struct {
	Name          string     `json:"name"`
	Project       string     `json:"project"`
	Team          string     `json:"team"`
	InitializedAt *time.Time `json:"initializedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	WorkflowRunID string     `json:"workflowRunID"`
	WorkflowID    string     `json:"workflowID"`
}

type ChainloopMaintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ChainloopRenderer struct {
	att     *v1.Attestation
	builder *builderInfo
}

type builderInfo struct {
	version, digest string
}

func newChainloopRenderer(att *v1.Attestation, builderVersion, builderDigest string) *ChainloopRenderer {
	return &ChainloopRenderer{att, &builderInfo{builderVersion, builderDigest}}
}

func (r *ChainloopRenderer) Predicate() (interface{}, error) {
	return ChainloopProvenancePredicateV1{
		Materials:  outputChainloopMaterials(r.att, false),
		BuildType:  chainloopBuildType,
		Builder:    &slsacommon.ProvenanceBuilder{ID: fmt.Sprintf(builderIDFmt, r.builder.version, r.builder.digest)},
		Metadata:   getChainloopMeta(r.att),
		Env:        r.att.EnvVars,
		RunnerType: r.att.GetRunnerType().String(),
		RunnerURL:  r.att.GetRunnerUrl(),
	}, nil
}

func getChainloopMeta(att *v1.Attestation) *ChainloopMetadata {
	initializedAt := att.InitializedAt.AsTime()
	wfMeta := att.GetWorkflow()

	// Finished at is set at the time of render
	finishedAt := time.Now()

	return &ChainloopMetadata{
		InitializedAt: &initializedAt,
		FinishedAt:    &finishedAt,
		Name:          wfMeta.GetName(),
		Team:          wfMeta.GetTeam(),
		Project:       wfMeta.GetProject(),
		WorkflowRunID: wfMeta.GetWorkflowRunId(),
		WorkflowID:    wfMeta.GetWorkflowId(),
	}
}

func (r *ChainloopRenderer) Header() (*in_toto.StatementHeader, error) {
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
		PredicateType: ChainloopPredicateTypeV1,
		Subject:       subjects,
	}, nil
}

func outputChainloopMaterials(att *v1.Attestation, onlyOutput bool) []*ChainloopProvenanceMaterial {
	// Sort material keys to stabilize output
	keys := make([]string, 0, len(att.GetMaterials()))
	for k := range att.GetMaterials() {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := []*ChainloopProvenanceMaterial{}
	materials := att.GetMaterials()
	for _, mdefName := range keys {
		mdef := materials[mdefName]

		var value, digest string
		artifactType := mdef.MaterialType
		var isOutput bool

		switch mdef.MaterialType {
		case schemaapi.CraftingSchema_Material_ARTIFACT, schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON, schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON:
			a := mdef.GetArtifact()
			value, digest, isOutput = a.Name, a.Digest, a.IsSubject
		case schemaapi.CraftingSchema_Material_CONTAINER_IMAGE:
			a := mdef.GetContainerImage()
			value, digest, isOutput = a.Name, a.Digest, a.IsSubject
		case schemaapi.CraftingSchema_Material_STRING:
			a := mdef.GetString_()
			value = a.Value
		}

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !isOutput {
			continue
		}

		material := &ChainloopProvenanceM{}
		if artifactType == schemaapi.CraftingSchema_Material_STRING {
			material.StringVal = value
		} else if digest != "" {
			parts := strings.Split(digest, ":")
			material.SLSA = &SLSACommonProvenanceMaterial{
				&slsacommon.ProvenanceMaterial{
					URI: value,
					Digest: map[string]string{
						parts[0]: parts[1],
					},
				},
			}
		}

		res = append(res, &ChainloopProvenanceMaterial{
			Material: material,
			Name:     mdefName,
			Type:     artifactType.String(),
		})
	}

	return res
}

// Extract the Chainloop attestation predicate from an encoded DSSE envelope
func ExtractPredicate(envelope *dsse.Envelope) (*ChainloopProvenancePredicateV1, error) {
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
	var predicate *ChainloopProvenancePredicateV1
	switch statement.PredicateType {
	case ChainloopPredicateTypeV1:
		if predicate, err = extractPredicateV1(statement); err != nil {
			return nil, fmt.Errorf("extracting predicate: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported predicate type: %s", statement.PredicateType)
	}

	return predicate, nil
}

func extractPredicateV1(statement *in_toto.Statement) (*ChainloopProvenancePredicateV1, error) {
	jsonPredicate, err := json.Marshal(statement.Predicate)
	if err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	predicate := &ChainloopProvenancePredicateV1{}
	if err := json.Unmarshal(jsonPredicate, predicate); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return predicate, nil
}
