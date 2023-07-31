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

package action

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	sigdsee "github.com/sigstore/sigstore/pkg/signature/dsse"
)

type WorkflowRunDescribe struct {
	cfg *ActionsOpts
}

type WorkflowRunItemFull struct {
	WorkflowRun *WorkflowRunItem            `json:"workflowRun"`
	Workflow    *WorkflowItem               `json:"workflow"`
	Attestation *WorkflowRunAttestationItem `json:"attestation,omitempty"`
	Verified    bool                        `json:"verified"`
}

type WorkflowRunAttestationItem struct {
	ID          string         `json:"id"`
	CreatedAt   *time.Time     `json:"createdAt"`
	Envelope    *dsse.Envelope `json:"envelope"`
	statement   *in_toto.Statement
	Materials   []*Material   `json:"materials,omitempty"`
	EnvVars     []*EnvVar     `json:"envvars,omitempty"`
	Annotations []*Annotation `json:"annotations,omitempty"`
}

type Material struct {
	Name string `json:"name"`
	// filename, container image name, string value, ...
	Value       string        `json:"value"`
	Hash        string        `json:"hash"`
	Type        string        `json:"type"`
	Annotations []*Annotation `json:"annotations,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Annotation struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (i *WorkflowRunAttestationItem) Statement() *in_toto.Statement {
	return i.statement
}

func NewWorkflowRunDescribe(cfg *ActionsOpts) *WorkflowRunDescribe {
	return &WorkflowRunDescribe{cfg}
}

func (action *WorkflowRunDescribe) Run(runID string, verify bool, publicKey string) (*WorkflowRunItemFull, error) {
	client := pb.NewWorkflowRunServiceClient(action.cfg.CPConnection)
	resp, err := client.View(context.Background(), &pb.WorkflowRunServiceViewRequest{Id: runID})
	if err != nil {
		return nil, err
	}

	wr := resp.GetResult().GetWorkflowRun()
	wf := wr.GetWorkflow()

	item := &WorkflowRunItemFull{
		WorkflowRun: pbWorkflowRunItemToAction(wr),
		Workflow:    pbWorkflowItemToAction(wf),
	}

	if wr.FinishedAt != nil {
		item.WorkflowRun.FinishedAt = toTimePtr(wr.FinishedAt.AsTime())
	}

	attestation := resp.GetResult().GetAttestation()
	// The item does not have associated attestation
	if attestation == nil {
		return item, nil
	}

	envelope, err := decodeEnvelope(attestation.Envelope)
	if err != nil {
		return nil, err
	}

	if verify {
		if err := verifyEnvelope(context.Background(), envelope, publicKey); err != nil {
			action.cfg.Logger.Debug().Err(err).Msg("verifying the envelope")
			return nil, errors.New("invalid signature, did you provide the right key?")
		}

		item.Verified = true
	}

	statement, err := chainloop.ExtractStatement(envelope)
	if err != nil {
		return nil, fmt.Errorf("extracting statement: %w", err)
	}

	envVars := make([]*EnvVar, 0, len(attestation.GetEnvVars()))
	for _, v := range attestation.GetEnvVars() {
		envVars = append(envVars, &EnvVar{Name: v.Name, Value: v.Value})
	}

	materials := make([]*Material, 0, len(attestation.GetMaterials()))
	for _, v := range attestation.GetMaterials() {
		materials = append(materials, materialPBToAction(v))
	}

	keys := make([]string, 0, len(attestation.GetAnnotations()))
	for k := range attestation.GetAnnotations() {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	annotations := make([]*Annotation, 0, len(attestation.GetAnnotations()))
	for _, k := range keys {
		annotations = append(annotations, &Annotation{
			Name: k, Value: attestation.GetAnnotations()[k],
		})
	}

	item.Attestation = &WorkflowRunAttestationItem{
		ID: attestation.Id, CreatedAt: toTimePtr(attestation.CreatedAt.AsTime()),
		Envelope:    envelope,
		statement:   statement,
		EnvVars:     envVars,
		Materials:   materials,
		Annotations: annotations,
	}

	return item, nil
}

func materialPBToAction(in *pb.AttestationItem_Material) *Material {
	m := &Material{
		Name:  in.Name,
		Value: in.Value,
		Type:  in.Type,
		Hash:  in.Hash,
	}

	// append annotations sorted
	if in.Annotations != nil {
		keys := make([]string, 0, len(in.Annotations))
		for k := range in.Annotations {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			m.Annotations = append(m.Annotations, &Annotation{Name: k, Value: in.Annotations[k]})
		}
	}

	return m
}

func verifyEnvelope(ctx context.Context, e *dsse.Envelope, publicKey string) error {
	// Currently we only support basic cosign public key check
	// TODO: Add more verification methods
	verifier, err := sigs.PublicKeyFromKeyRef(ctx, publicKey)
	if err != nil {
		return err
	}

	dsseVerifier, err := dsse.NewEnvelopeVerifier(&sigdsee.VerifierAdapter{SignatureVerifier: verifier})
	if err != nil {
		return err
	}

	_, err = dsseVerifier.Verify(ctx, e)
	return err
}
