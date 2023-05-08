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
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	"github.com/in-toto/in-toto-golang/in_toto"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

// // Replace custom material type with https://github.com/in-toto/attestation/blob/main/spec/v1.0/resource_descriptor.md
// const ChainloopPredicateTypeV02 = "chainloop.dev/attestation/v0.2"

// TODO: Figure out a more appropriate meaning
const chainloopBuildType = "chainloop.dev/workflowrun/v0.1"

const builderIDFmt = "chainloop.dev/cli/%s@%s"

// NormalizablePredicate represents a common interface of how to extract materials and env vars
type NormalizablePredicate interface {
	GetEnvVars() map[string]string
	GetMaterials() []*NormalizedMaterial
}

type NormalizedMaterial struct {
	Name        string
	Type        string
	StringValue string
}

type ProvenancePredicateVersions struct {
	V01 *ProvenancePredicateV01
	V02 *ProvenancePredicateV02
}

type ProvenancePredicateCommon struct {
	Metadata   *Metadata                     `json:"metadata"`
	Builder    *slsacommon.ProvenanceBuilder `json:"builder"`
	BuildType  string                        `json:"buildType"`
	Env        map[string]string             `json:"env,omitempty"`
	RunnerType string                        `json:"runnerType"`
	RunnerURL  string                        `json:"runnerURL,omitempty"`
}

type Metadata struct {
	Name          string     `json:"name"`
	Project       string     `json:"project"`
	Team          string     `json:"team"`
	InitializedAt *time.Time `json:"initializedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	WorkflowRunID string     `json:"workflowRunID"`
	WorkflowID    string     `json:"workflowID"`
}

type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type builderInfo struct {
	version, digest string
}

type RendererCommon struct {
	predicateType string
	att           *v1.Attestation
	builder       *builderInfo
}

func predicateCommon(builderInfo *builderInfo, att *v1.Attestation) *ProvenancePredicateCommon {
	return &ProvenancePredicateCommon{
		BuildType:  chainloopBuildType,
		Builder:    &slsacommon.ProvenanceBuilder{ID: fmt.Sprintf(builderIDFmt, builderInfo.version, builderInfo.digest)},
		Metadata:   getChainloopMeta(att),
		Env:        att.EnvVars,
		RunnerType: att.GetRunnerType().String(),
		RunnerURL:  att.GetRunnerUrl(),
	}
}

func getChainloopMeta(att *v1.Attestation) *Metadata {
	initializedAt := att.InitializedAt.AsTime()
	wfMeta := att.GetWorkflow()

	// Finished at is set at the time of render
	finishedAt := time.Now()

	return &Metadata{
		InitializedAt: &initializedAt,
		FinishedAt:    &finishedAt,
		Name:          wfMeta.GetName(),
		Team:          wfMeta.GetTeam(),
		Project:       wfMeta.GetProject(),
		WorkflowRunID: wfMeta.GetWorkflowRunId(),
		WorkflowID:    wfMeta.GetWorkflowId(),
	}
}

func ExtractStatement(envelope *dsse.Envelope) (*in_toto.Statement, error) {
	decodedPayload, err := envelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	// 1 - Extract the in-toto statement
	statement := &in_toto.Statement{}
	if err := json.Unmarshal(decodedPayload, statement); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return statement, nil
}

// Extract the Chainloop attestation predicate from an encoded DSSE envelope
func ExtractPredicate(envelope *dsse.Envelope) (*ProvenancePredicateVersions, error) {
	// 1 - Extract the in-toto statement
	statement, err := ExtractStatement(envelope)
	if err != nil {
		return nil, fmt.Errorf("extracting statement: %w", err)
	}

	// 2 - Extract the Chainloop predicate from the in-toto statement
	switch statement.PredicateType {
	case PredicateTypeV01:
		var predicate *ProvenancePredicateV01
		if err = extractPredicate(statement, &predicate); err != nil {
			return nil, fmt.Errorf("extracting predicate: %w", err)
		}

		return &ProvenancePredicateVersions{V01: predicate}, nil
	case PredicateTypeV02:
		var predicate *ProvenancePredicateV02
		if err = extractPredicate(statement, &predicate); err != nil {
			return nil, fmt.Errorf("extracting predicate: %w", err)
		}

		return &ProvenancePredicateVersions{V02: predicate}, nil
	default:
		return nil, fmt.Errorf("unsupported predicate type: %s", statement.PredicateType)
	}
}

func extractPredicate(statement *in_toto.Statement, v any) error {
	jsonPredicate, err := json.Marshal(statement.Predicate)
	if err != nil {
		return fmt.Errorf("un-marshaling predicate: %w", err)
	}

	if err := json.Unmarshal(jsonPredicate, v); err != nil {
		return fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return nil
}

// Implement NormalizablePredicate interface
func (p *ProvenancePredicateCommon) GetEnvVars() map[string]string {
	return p.Env
}
