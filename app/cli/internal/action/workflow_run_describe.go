//
// Copyright 2024 The Chainloop Authors.
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
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"sort"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	intoto "github.com/in-toto/attestation/go/v1"
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
	Envelope    *dsse.Envelope `json:"envelope"`
	statement   *intoto.Statement
	Materials   []*Material   `json:"materials,omitempty"`
	EnvVars     []*EnvVar     `json:"envvars,omitempty"`
	Annotations []*Annotation `json:"annotations,omitempty"`
	// Digest in CAS backend
	Digest string `json:"digest"`
}

type Material struct {
	Name           string        `json:"name"`
	Value          string        `json:"value"`
	Hash           string        `json:"hash"`
	Tag            string        `json:"tag"`
	Filename       string        `json:"filename"`
	Type           string        `json:"type"`
	Annotations    []*Annotation `json:"annotations,omitempty"`
	UploadedToCAS  bool          `json:"uploadedToCAS,omitempty"`
	EmbeddedInline bool          `json:"embeddedInline,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Annotation struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (i *WorkflowRunAttestationItem) Statement() *intoto.Statement {
	return i.statement
}

func NewWorkflowRunDescribe(cfg *ActionsOpts) *WorkflowRunDescribe {
	return &WorkflowRunDescribe{cfg}
}

type WorkflowRunDescribeOpts struct {
	RunID, Digest           string
	Verify                  bool
	PublicKeyRef            string
	CertPath, CertChainPath string
}

func (action *WorkflowRunDescribe) Run(ctx context.Context, opts *WorkflowRunDescribeOpts) (*WorkflowRunItemFull, error) {
	client := pb.NewWorkflowRunServiceClient(action.cfg.CPConnection)

	req := &pb.WorkflowRunServiceViewRequest{}
	if opts.Digest != "" {
		req.Ref = &pb.WorkflowRunServiceViewRequest_Digest{Digest: opts.Digest}
	} else if opts.RunID != "" {
		req.Ref = &pb.WorkflowRunServiceViewRequest_Id{Id: opts.RunID}
	}

	resp, err := client.View(ctx, req)
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

	if opts.Verify {
		if err := verifyEnvelope(ctx, envelope, opts); err != nil {
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
		Envelope:    envelope,
		statement:   statement,
		EnvVars:     envVars,
		Materials:   materials,
		Annotations: annotations,
		Digest:      attestation.DigestInCasBackend,
	}

	return item, nil
}

func materialPBToAction(in *pb.AttestationItem_Material) *Material {
	m := &Material{
		Name:           in.Name,
		Value:          in.Value,
		Type:           in.Type,
		Hash:           in.Hash,
		Tag:            in.Tag,
		UploadedToCAS:  in.UploadedToCas,
		Filename:       in.Filename,
		EmbeddedInline: in.EmbeddedInline,
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

func verifyEnvelope(ctx context.Context, e *dsse.Envelope, opts *WorkflowRunDescribeOpts) error {
	if opts.PublicKeyRef == "" && opts.CertPath == "" {
		return fmt.Errorf("no public key or cert path specified")
	}

	var verifier signature.Verifier
	var err error
	if opts.PublicKeyRef != "" {
		verifier, err = sigs.PublicKeyFromKeyRef(ctx, opts.PublicKeyRef)
		if err != nil {
			return fmt.Errorf("invalid public key: %w", err)
		}
	}

	if opts.CertPath != "" {
		// Load cert from PEM
		certs, err := loadCertificates(opts.CertPath)
		if err != nil {
			return fmt.Errorf("loading certificate: %w", err)
		}

		var chain []*x509.Certificate
		if opts.CertChainPath != "" {
			chain, err = loadCertificates(opts.CertChainPath)
			if err != nil {
				return fmt.Errorf("loading certificate chain: %w", err)
			}
		}

		verifier, err = cosign.ValidateAndUnpackCertWithChain(certs[0], chain, &cosign.CheckOpts{IgnoreSCT: true})
		if err != nil {
			return fmt.Errorf("validating the certificate: %w", err)
		}
	}

	dsseVerifier, err := dsse.NewEnvelopeVerifier(&sigdsee.VerifierAdapter{SignatureVerifier: verifier})
	if err != nil {
		return fmt.Errorf("creating DSSE verifier: %w", err)
	}

	_, err = dsseVerifier.Verify(ctx, e)
	return err
}

func loadCertificates(certPath string) ([]*x509.Certificate, error) {
	cert, err := blob.LoadFileOrURL(certPath)
	if err != nil {
		return nil, fmt.Errorf("loading certificate: %w", err)
	}
	certs, err := cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(cert))
	if err != nil {
		return nil, fmt.Errorf("loading certificates: %w", err)
	}

	return certs, nil
}
