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

	"github.com/in-toto/in-toto-golang/in_toto"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

// // Replace custom material type with https://github.com/in-toto/attestation/blob/main/spec/v1.0/resource_descriptor.md
// const ChainloopPredicateTypeV02 = "chainloop.dev/attestation/v0.2"

// TODO: Figure out a more appropriate meaning
const chainloopBuildType = "chainloop.dev/workflowrun/v0.1"

const builderIDFmt = "chainloop.dev/cli/%s@%s"

type ProvenancePredicateVersions struct {
	V01 *ProvenancePredicateV01
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
