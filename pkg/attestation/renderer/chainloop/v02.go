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

package chainloop

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// Replace custom material type with https://github.com/in-toto/attestation/blob/main/spec/v1.0/resource_descriptor.md
const PredicateTypeV02 = "chainloop.dev/attestation/v0.2"
const AttPolicyEvaluation = "CHAINLOOP.ATTESTATION"

type ProvenancePredicateV02 struct {
	*ProvenancePredicateCommon
	Materials []*intoto.ResourceDescriptor `json:"materials,omitempty"`
	// Map materials and policies
	PolicyEvaluations map[string][]*PolicyEvaluation `json:"policyEvaluations,omitempty"`
	// Used to read policy evaluations from old attestations
	PolicyEvaluationsFallback map[string][]*PolicyEvaluation `json:"policy_evaluations,omitempty"`

	// Whether the attestation has policy violations
	PolicyHasViolations bool `json:"policyHasViolations"`
	// Whether we want to block the attestation on policy violations
	PolicyCheckBlockingStrategy PolicyViolationBlockingStrategy `json:"policyCheckBlockingStrategy"`
	// Whether the policy check was bypassed
	PolicyBlockBypassEnabled bool `json:"policyBlockBypassEnabled"`
	// Whether the attestation was blocked due to policy violations
	PolicyAttBlocked bool `json:"policyAttBlocked"`
}

type PolicyViolationBlockingStrategy string

const (
	PolicyViolationBlockingStrategyEnforced PolicyViolationBlockingStrategy = "ENFORCED"
	PolicyViolationBlockingStrategyAdvisory PolicyViolationBlockingStrategy = "ADVISORY"
)

type PolicyEvaluation struct {
	Name         string `json:"name"`
	MaterialName string `json:"materialName,omitempty"`
	// Needed to read old attestations
	MaterialNameFallback string                     `json:"material_name,omitempty"`
	Body                 string                     `json:"body,omitempty"`
	Sources              []string                   `json:"sources,omitempty"`
	PolicyReference      *intoto.ResourceDescriptor `json:"policyReference,omitempty"`
	// Support old attestations
	PolicyReferenceFallback *intoto.ResourceDescriptor `json:"policy_reference,omitempty"`
	Description             string                     `json:"description,omitempty"`
	Annotations             map[string]string          `json:"annotations,omitempty"`
	Violations              []*PolicyViolation         `json:"violations,omitempty"`
	With                    map[string]string          `json:"with,omitempty"`
	Type                    string                     `json:"type"`
	Skipped                 bool                       `json:"skipped"`
	SkipReasons             []string                   `json:"skipReasons,omitempty"`
	GroupReference          *intoto.ResourceDescriptor `json:"groupReference,omitempty"`
	Requirements            []string                   `json:"requirements,omitempty"`
}

type PolicyViolation struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type RendererV02 struct {
	*RendererCommon
	schema    *schemaapi.CraftingSchema
	attClient pb.AttestationServiceClient
	logger    *zerolog.Logger
}

func NewChainloopRendererV02(att *v1.Attestation, schema *schemaapi.CraftingSchema, builderVersion, builderDigest string, attClient pb.AttestationServiceClient, logger *zerolog.Logger) *RendererV02 {
	return &RendererV02{
		&RendererCommon{
			PredicateTypeV02, att, &builderInfo{builderVersion, builderDigest},
		},
		schema,
		attClient,
		logger,
	}
}

func (r *RendererV02) Statement(_ context.Context) (*intoto.Statement, error) {
	subject, err := r.subject()
	if err != nil {
		return nil, fmt.Errorf("error creating subject: %w", err)
	}

	predicate, err := r.predicate()
	if err != nil {
		return nil, fmt.Errorf("error creating predicate: %w", err)
	}

	statement := &intoto.Statement{
		Type:          intoto.StatementTypeUri,
		Subject:       subject,
		PredicateType: r.predicateType,
		Predicate:     predicate,
	}

	return statement, nil
}

func commitAnnotations(c *v1.Commit) (*structpb.Struct, error) {
	annotationsRaw := map[string]interface{}{
		subjectGitAnnotationWhen:        c.GetDate().AsTime().Format(time.RFC3339),
		subjectGitAnnotationAuthorEmail: c.GetAuthorEmail(),
		subjectGitAnnotationAuthorName:  c.GetAuthorName(),
		subjectGitAnnotationMessage:     c.GetMessage(),
	}

	// add signature only if exists
	if c.GetSignature() != "" {
		annotationsRaw[subjectGitAnnotationSignature] = c.GetSignature()
	}

	if remotes := c.GetRemotes(); len(remotes) > 0 {
		remotesRaw := []interface{}{}
		for _, r := range remotes {
			remotesRaw = append(remotesRaw, map[string]interface{}{
				"name": r.GetName(),
				"url":  r.GetUrl(),
			})
		}

		annotationsRaw[subjectGitAnnotationRemotes] = remotesRaw
	}

	return structpb.NewStruct(annotationsRaw)
}

func (r *RendererV02) subject() ([]*intoto.ResourceDescriptor, error) {
	raw, err := json.Marshal(r.att)
	if err != nil {
		return nil, err
	}

	// We might don't want this and just force the existence of one material with output = true
	subject := []*intoto.ResourceDescriptor{
		{
			Name: v1.CreateAnnotation(fmt.Sprintf("workflow.%s", r.att.GetWorkflow().Name)),
			Digest: map[string]string{
				"sha256": fmt.Sprintf("%x", sha256.Sum256(raw)),
			},
		},
	}

	if head := r.att.GetHead(); head != nil {
		annotations, err := commitAnnotations(head)
		if err != nil {
			return nil, fmt.Errorf("error creating annotations: %w", err)
		}

		subject = append(subject, &intoto.ResourceDescriptor{
			Name:        SubjectGitHead,
			Digest:      map[string]string{"sha1": head.GetHash()},
			Annotations: annotations,
		})
	}

	normalizedMaterials, err := outputMaterials(r.att, true)
	if err != nil {
		return nil, fmt.Errorf("error normalizing materials: %w", err)
	}

	for _, m := range normalizedMaterials {
		if m.Digest != nil {
			subject = append(subject, &intoto.ResourceDescriptor{
				Name:        m.Name,
				Digest:      m.Digest,
				Annotations: m.Annotations,
			})
		}
	}

	return subject, nil
}

func (r *RendererV02) predicate() (*structpb.Struct, error) {
	normalizedMaterials, err := outputMaterials(r.att, false)
	if err != nil {
		return nil, fmt.Errorf("error normalizing materials: %w", err)
	}

	policies, hasViolations, err := mappedPolicyEvaluations(r.att)
	if err != nil {
		return nil, fmt.Errorf("error rendering policy evaluations: %w", err)
	}

	policyCheckBlockingStrategy := PolicyViolationBlockingStrategyAdvisory
	if r.att.GetBlockOnPolicyViolation() {
		policyCheckBlockingStrategy = PolicyViolationBlockingStrategyEnforced
	}

	p := ProvenancePredicateV02{
		ProvenancePredicateCommon:   predicateCommon(r.builder, r.att),
		Materials:                   normalizedMaterials,
		PolicyEvaluations:           policies,
		PolicyHasViolations:         hasViolations,
		PolicyCheckBlockingStrategy: policyCheckBlockingStrategy,
		PolicyBlockBypassEnabled:    r.att.GetBypassPolicyCheck(),
		PolicyAttBlocked:            hasViolations && r.att.GetBlockOnPolicyViolation() && !r.att.GetBypassPolicyCheck(),
	}

	// transform to structpb.Struct in a two steps process
	// 1 - ProvenancePredicate -> json
	// 2 - json -> structpb.Struct
	predicateJSON, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("error marshaling predicate: %w", err)
	}

	predicate := &structpb.Struct{}
	if err := protojson.Unmarshal(predicateJSON, predicate); err != nil {
		return nil, fmt.Errorf("error unmarshaling predicate: %w", err)
	}

	return predicate, nil
}

// collect all policy evaluations grouped by material and returns if there is a policy violation
func mappedPolicyEvaluations(att *v1.Attestation) (map[string][]*PolicyEvaluation, bool, error) {
	var hasPolicyViolations bool
	result := map[string][]*PolicyEvaluation{}

	for _, p := range att.GetPolicyEvaluations() {
		keyName := p.MaterialName
		if keyName == "" {
			keyName = AttPolicyEvaluation
		}

		ev, err := renderEvaluation(p)
		if err != nil {
			return nil, false, err
		}

		if len(ev.Violations) > 0 {
			hasPolicyViolations = true
		}

		result[keyName] = append(result[keyName], ev)
	}

	return result, hasPolicyViolations, nil
}

func renderEvaluation(ev *v1.PolicyEvaluation) (*PolicyEvaluation, error) {
	// Map violations
	violations := make([]*PolicyViolation, 0)
	for _, vi := range ev.Violations {
		violations = append(violations, &PolicyViolation{
			Subject: vi.Subject,
			Message: vi.Message,
		})
	}

	policyRef, err := renderReference(ev.GetPolicyReference())
	if err != nil {
		return nil, err
	}

	groupRef, err := renderReference(ev.GetGroupReference())
	if err != nil {
		return nil, err
	}

	return &PolicyEvaluation{
		Name:            ev.Name,
		MaterialName:    ev.MaterialName,
		Body:            ev.Body,
		Sources:         ev.Sources,
		Annotations:     ev.Annotations,
		Description:     ev.Description,
		With:            ev.With,
		Type:            ev.Type.String(),
		Violations:      violations,
		PolicyReference: policyRef,
		SkipReasons:     ev.SkipReasons,
		Skipped:         ev.Skipped,
		GroupReference:  groupRef,
		Requirements:    ev.Requirements,
	}, nil
}

func renderReference(ref *v1.PolicyEvaluation_Reference) (*intoto.ResourceDescriptor, error) {
	// skip empty references
	if ref == nil {
		return nil, nil
	}

	annotations, err := structpb.NewStruct(map[string]interface{}{"name": ref.GetName(), "organization": ref.GetOrgName()})
	if err != nil {
		// Struct raise errors in some conditions (when a field is not UTF8, for example). We need to handle them, although it's a remote possibility
		return nil, err
	}
	return &intoto.ResourceDescriptor{
		Name: ref.GetName(),
		Uri:  ref.GetUri(),
		Digest: map[string]string{
			"sha256": strings.TrimPrefix(ref.GetDigest(), "sha256:"),
		},
		Annotations: annotations,
	}, nil
}

func outputMaterials(att *v1.Attestation, onlyOutput bool) ([]*intoto.ResourceDescriptor, error) {
	// Sort material keys to stabilize output
	keys := make([]string, 0, len(att.GetMaterials()))
	for k := range att.GetMaterials() {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := []*intoto.ResourceDescriptor{}
	materials := att.GetMaterials()
	for _, mdefName := range keys {
		mdef := materials[mdefName]

		nMaterial, err := mdef.NormalizedOutput()
		if err != nil {
			return nil, fmt.Errorf("error normalizing material: %w", err)
		}

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material, err := mdef.CraftingStateToIntotoDescriptor(mdefName)
		if err != nil {
			return nil, fmt.Errorf("rendering material: %w", err)
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

func (p *ProvenancePredicateV02) GetPolicyEvaluations() map[string][]*PolicyEvaluation {
	return p.PolicyEvaluations
}

func (p *ProvenancePredicateV02) GetPolicyEvaluationStatus() *PolicyEvaluationStatus {
	return &PolicyEvaluationStatus{
		Strategy:      p.PolicyCheckBlockingStrategy,
		Bypassed:      p.PolicyBlockBypassEnabled,
		Blocked:       p.PolicyAttBlocked,
		HasViolations: p.PolicyHasViolations,
	}
}

// Translate a ResourceDescriptor to a NormalizedMaterial
func normalizeMaterial(material *intoto.ResourceDescriptor) (*NormalizedMaterial, error) {
	m := &NormalizedMaterial{
		ReferencedSourceComponent: &ReferencedSourceComponent{},
	}

	// Set custom annotations
	m.Annotations = make(map[string]string)
	mAnnotationsMap := material.Annotations.GetFields()
	for k, v := range mAnnotationsMap {
		// if the annotation key doesn't start with chainloop.
		// we set it as a custom annotation
		if strings.HasPrefix(k, v1.AnnotationPrefix) {
			continue
		}

		m.Annotations[k] = v.GetStringValue()
	}

	mType, ok := mAnnotationsMap[v1.AnnotationMaterialType]
	if !ok {
		return nil, fmt.Errorf("material type not found")
	}

	// Set the type
	m.Type = mType.GetStringValue()

	mName, ok := mAnnotationsMap[v1.AnnotationMaterialName]
	if !ok {
		return nil, fmt.Errorf("material name not found")
	}

	// Set the Material Name
	m.Name = mName.GetStringValue()

	// Set the Value
	// If we have a string material, we just set the value
	if m.Type == schemaapi.CraftingSchema_Material_STRING.String() {
		if material.Content == nil {
			return nil, fmt.Errorf("material content not found")
		}

		m.Value = string(material.Content)
		hash, ok := material.Digest["sha256"]
		if ok {
			m.Hash = &crv1.Hash{Algorithm: "sha256", Hex: hash}
		}

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

	if v, ok := mAnnotationsMap[v1.AnnotationMaterialCAS]; ok && v.GetBoolValue() {
		m.UploadedToCAS = true
	}

	if v, ok := mAnnotationsMap[v1.AnnotationMaterialInlineCAS]; ok && v.GetBoolValue() {
		m.EmbeddedInline = true
	}

	// Extract the container image tag if it's set in the annotations
	if v, ok := mAnnotationsMap[v1.AnnotationContainerTag]; ok && v.GetStringValue() != "" {
		m.Tag = v.GetStringValue()
	}

	// Extract the referenced source component
	if v, ok := mAnnotationsMap[v1.AnnotationsSBOMMainComponentName]; ok && v.GetStringValue() != "" {
		m.ReferencedSourceComponent.Name = v.GetStringValue()
	}

	if v, ok := mAnnotationsMap[v1.AnnotationsSBOMMainComponentVersion]; ok && v.GetStringValue() != "" {
		m.ReferencedSourceComponent.Version = v.GetStringValue()
	}

	if v, ok := mAnnotationsMap[v1.AnnotationsSBOMMainComponentType]; ok && v.GetStringValue() != "" {
		m.ReferencedSourceComponent.Type = v.GetStringValue()
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
