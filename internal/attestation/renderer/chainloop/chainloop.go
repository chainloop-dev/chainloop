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
	"google.golang.org/protobuf/encoding/protojson"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	intoto "github.com/in-toto/attestation/go/v1"
)

// TODO: Figure out a more appropriate meaning
const chainloopBuildType = "chainloop.dev/workflowrun/v0.1"

const builderIDFmt = "chainloop.dev/cli/%s@%s"

// NormalizablePredicate represents a common interface of how to extract materials and env vars
type NormalizablePredicate interface {
	GetAnnotations() map[string]string
	GetEnvVars() map[string]string
	GetMaterials() []*NormalizedMaterial
	GetRunLink() string
}

type NormalizedMaterial struct {
	// Name of the Material
	Name string
	// Type of the Material
	Type string
	// filename of the artifact that was either uploaded or injected inline in "value"
	Filename string
	// Inline content for an artifact or string material
	Value string
	// Hash of the Material
	Hash *crv1.Hash
	// Whether the Material was uploaded and available for download from CAS
	UploadedToCAS bool
	// Whether the Material was embedded inline in the attestation
	EmbeddedInline bool
	// Custom annotations
	Annotations map[string]string
}

type ProvenancePredicateCommon struct {
	Metadata   *Metadata         `json:"metadata"`
	Builder    *builder          `json:"builder"`
	BuildType  string            `json:"buildType"`
	Env        map[string]string `json:"env,omitempty"`
	RunnerType string            `json:"runnerType"`
	RunnerURL  string            `json:"runnerURL,omitempty"`
	// Custom annotations
	Annotations map[string]string `json:"annotations,omitempty"`
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

type builder struct {
	ID string `json:"id"`
}

type RendererCommon struct {
	predicateType string
	att           *v1.Attestation
	builder       *builderInfo
}

func predicateCommon(builderInfo *builderInfo, att *v1.Attestation) *ProvenancePredicateCommon {
	return &ProvenancePredicateCommon{
		BuildType:   chainloopBuildType,
		Builder:     &builder{ID: fmt.Sprintf(builderIDFmt, builderInfo.version, builderInfo.digest)},
		Metadata:    getChainloopMeta(att),
		Env:         att.EnvVars,
		RunnerType:  att.GetRunnerType().String(),
		RunnerURL:   att.GetRunnerUrl(),
		Annotations: att.Annotations,
	}
}

func getChainloopMeta(att *v1.Attestation) *Metadata {
	initializedAt := att.InitializedAt.AsTime()
	finishedAt := att.GetFinishedAt().AsTime()
	wfMeta := att.GetWorkflow()

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

func ExtractStatement(envelope *dsse.Envelope) (*intoto.Statement, error) {
	decodedPayload, err := envelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	// 1 - Extract the in-toto statement
	statement := &intoto.Statement{}
	if err := protojson.Unmarshal(decodedPayload, statement); err != nil {
		return nil, fmt.Errorf("un-marshaling predicate: %w", err)
	}

	return statement, nil
}

// Extract the Chainloop attestation predicate from an encoded DSSE envelope
// NOTE: We return a NormalizablePredicate interface to allow for future versions
// of the predicate to be extracted without updating the consumer.
// Yes, having the producer define and return an interface is an anti-pattern.
// but it greatly simplifies the code since there are multiple consumers at different layers of the app
// and we expect predicates to evolve quickly
func ExtractPredicate(envelope *dsse.Envelope) (NormalizablePredicate, error) {
	// 1 - Extract the in-toto statement
	statement, err := ExtractStatement(envelope)
	if err != nil {
		return nil, fmt.Errorf("extracting statement: %w", err)
	}

	// 2 - Extract the Chainloop predicate from the in-toto statement
	switch statement.PredicateType {
	case PredicateTypeV02:
		var predicate *ProvenancePredicateV02
		if err = extractPredicate(statement, &predicate); err != nil {
			return nil, fmt.Errorf("extracting predicate: %w", err)
		}

		return predicate, nil
	default:
		return nil, fmt.Errorf("unsupported predicate type: %s", statement.PredicateType)
	}
}

func extractPredicate(statement *intoto.Statement, v any) error {
	jsonPredicate, err := protojson.Marshal(statement.Predicate)
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

func (p *ProvenancePredicateCommon) GetRunLink() string {
	return p.RunnerURL
}

func (p *ProvenancePredicateCommon) GetAnnotations() map[string]string {
	return p.Annotations
}

const (
	rendererPrefix = "chainloop."
	// Subject names
	subjectGitHead = "git.head"
)

var (
	annotationMaterialType      = prefixed("material.type")
	annotationMaterialName      = prefixed("material.name")
	annotationMaterialCAS       = prefixed("material.cas")
	annotationMaterialInlineCAS = prefixed("material.cas.inline")
)

func prefixed(name string) string {
	return fmt.Sprintf("%s%s", rendererPrefix, name)
}
