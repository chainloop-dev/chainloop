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
	"github.com/chainloop-dev/chainloop/pkg/policies"
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
	PolicyEvaluations map[string][]*PolicyEvaluation `json:"policy_evaluations,omitempty"`
}

type PolicyEvaluation struct {
	Name            string                     `json:"name"`
	MaterialName    string                     `json:"material_name,omitempty"`
	Body            string                     `json:"body,omitempty"`
	Sources         []string                   `json:"sources,omitempty"`
	PolicyReference *intoto.ResourceDescriptor `json:"policy_reference,omitempty"`
	Description     string                     `json:"description,omitempty"`
	Annotations     map[string]string          `json:"annotations,omitempty"`
	Violations      []*PolicyViolation         `json:"violations,omitempty"`
	With            map[string]string          `json:"with,omitempty"`
	Type            string                     `json:"type"`
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

func (r *RendererV02) Statement(ctx context.Context) (*intoto.Statement, error) {
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

	// validate attestation-level policies
	pv := policies.NewPolicyVerifier(r.schema, r.attClient, r.logger)
	policyResults, err := pv.VerifyStatement(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("applying policies to statement: %w", err)
	}
	// log policy violations
	policies.LogPolicyViolations(policyResults, r.logger)

	// insert attestation level policy results into statement
	if err = addPolicyResults(statement, policyResults); err != nil {
		return nil, fmt.Errorf("adding policy results to statement: %w", err)
	}

	return statement, nil
}

// addPolicyResults adds policy evaluation results to the statement. It does it by deserializing the predicate from a structpb.Struct,
// filling PolicyEvaluations, and serializing it again to a structpb.Struct object, using JSON as an intermediate representation.
// Note that this is needed because intoto predicates are generic structpb.Struct
func addPolicyResults(statement *intoto.Statement, policyResults []*v1.PolicyEvaluation) error {
	if len(policyResults) == 0 {
		return nil
	}

	predicate := statement.Predicate
	// marshall to json
	jsonPredicate, err := protojson.Marshal(predicate)
	if err != nil {
		return fmt.Errorf("marshalling predicate: %w", err)
	}

	// unmarshall to our typed predicate object
	var p ProvenancePredicateV02
	err = json.Unmarshal(jsonPredicate, &p)
	if err != nil {
		return fmt.Errorf("unmarshalling predicate: %w", err)
	}

	// insert policy evaluations for attestation
	if p.PolicyEvaluations == nil {
		p.PolicyEvaluations = make(map[string][]*PolicyEvaluation)
	}
	attEvaluations := make([]*PolicyEvaluation, 0, len(policyResults))
	for _, ev := range policyResults {
		attEvaluations = append(attEvaluations, renderEvaluation(ev))
	}
	p.PolicyEvaluations[AttPolicyEvaluation] = attEvaluations

	// marshall back to JSON
	jsonPredicate, err = json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshalling predicate: %w", err)
	}

	// finally unmarshal from JSON to structpb.Struct.
	var finalPredicate structpb.Struct
	err = protojson.Unmarshal(jsonPredicate, &finalPredicate)
	if err != nil {
		return fmt.Errorf("unmarshalling predicate: %w", err)
	}

	statement.Predicate = &finalPredicate

	return nil
}

func commitAnnotations(c *v1.Commit) (*structpb.Struct, error) {
	annotationsRaw := map[string]interface{}{
		subjectGitAnnotationWhen:        c.GetDate().AsTime().Format(time.RFC3339),
		subjectGitAnnotationAuthorEmail: c.GetAuthorEmail(),
		subjectGitAnnotationAuthorName:  c.GetAuthorName(),
		subjectGitAnnotationMessage:     c.GetMessage(),
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
			Name: prefixed(fmt.Sprintf("workflow.%s", r.att.GetWorkflow().Name)),
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

	policies := policyEvaluationsFromMaterials(r.att)

	p := ProvenancePredicateV02{
		ProvenancePredicateCommon: predicateCommon(r.builder, r.att),
		Materials:                 normalizedMaterials,
		PolicyEvaluations:         policies,
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

// collect all policy evaluations grouped by material
func policyEvaluationsFromMaterials(att *v1.Attestation) map[string][]*PolicyEvaluation {
	result := map[string][]*PolicyEvaluation{}
	for _, p := range att.GetPolicyEvaluations() {
		result[p.MaterialName] = append(result[p.MaterialName], renderEvaluation(p))
	}

	return result
}

func renderEvaluation(ev *v1.PolicyEvaluation) *PolicyEvaluation {
	// Map violations
	violations := make([]*PolicyViolation, 0)
	for _, vi := range ev.Violations {
		violations = append(violations, &PolicyViolation{
			Subject: vi.Subject,
			Message: vi.Message,
		})
	}
	return &PolicyEvaluation{
		Name:         ev.Name,
		MaterialName: ev.MaterialName,
		Body:         ev.Body,
		Sources:      ev.Sources,
		Annotations:  ev.Annotations,
		Description:  ev.Description,
		With:         ev.With,
		Type:         ev.Type.String(),
		Violations:   violations,
		PolicyReference: &intoto.ResourceDescriptor{
			Name: ev.ReferenceName,
			Digest: map[string]string{
				"sha256": ev.ReferenceDigest,
			},
		},
	}
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

		artifactType := mdef.MaterialType
		nMaterial, err := mdef.NormalizedOutput()
		if err != nil {
			return nil, fmt.Errorf("error normalizing material: %w", err)
		}

		// Skip if we are expecting to show only the materials marked as output
		if onlyOutput && !nMaterial.IsOutput {
			continue
		}

		material := &intoto.ResourceDescriptor{}
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
		annotationsM := map[string]interface{}{
			AnnotationMaterialType: artifactType.String(),
			AnnotationMaterialName: mdefName,
		}

		// Set the special annotations for container images
		if artifactType == schemaapi.CraftingSchema_Material_CONTAINER_IMAGE {
			if tag := mdef.GetContainerImage().GetTag(); tag != "" {
				annotationsM[annotationContainerTag] = tag
			}

			if sigDigest := mdef.GetContainerImage().GetSignatureDigest(); sigDigest != "" {
				annotationsM[annotationSignatureDigest] = sigDigest
			}

			if sigProvider := mdef.GetContainerImage().GetSignatureProvider(); sigProvider != "" {
				annotationsM[annotationSignatureProvider] = sigProvider
			}

			if sigPayload := mdef.GetContainerImage().GetSignature(); sigPayload != "" {
				annotationsM[annotationMaterialSignature] = sigPayload
			}
		}

		// Custom annotations, it does not override the built-in ones
		for k, v := range mdef.Annotations {
			_, ok := annotationsM[k]
			if !ok {
				annotationsM[k] = v
			}
		}

		if mdef.UploadedToCas {
			annotationsM[AnnotationMaterialCAS] = true
		} else if mdef.InlineCas {
			annotationsM[annotationMaterialInlineCAS] = true
		}

		material.Annotations, err = structpb.NewStruct(annotationsM)
		if err != nil {
			return nil, fmt.Errorf("error creating annotations: %w", err)
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

// Translate a ResourceDescriptor to a NormalizedMaterial
func normalizeMaterial(material *intoto.ResourceDescriptor) (*NormalizedMaterial, error) {
	m := &NormalizedMaterial{}

	// Set custom annotations
	m.Annotations = make(map[string]string)
	mAnnotationsMap := material.Annotations.GetFields()
	for k, v := range mAnnotationsMap {
		// if the annotation key doesn't start with chainloop.
		// we set it as a custom annotation
		if strings.HasPrefix(k, rendererPrefix) {
			continue
		}

		m.Annotations[k] = v.GetStringValue()
	}

	mType, ok := mAnnotationsMap[AnnotationMaterialType]
	if !ok {
		return nil, fmt.Errorf("material type not found")
	}

	// Set the type
	m.Type = mType.GetStringValue()

	mName, ok := mAnnotationsMap[AnnotationMaterialName]
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

	if v, ok := mAnnotationsMap[AnnotationMaterialCAS]; ok && v.GetBoolValue() {
		m.UploadedToCAS = true
	}

	if v, ok := mAnnotationsMap[annotationMaterialInlineCAS]; ok && v.GetBoolValue() {
		m.EmbeddedInline = true
	}

	// Extract the container image tag if it's set in the annotations
	if v, ok := mAnnotationsMap[annotationContainerTag]; ok && v.GetStringValue() != "" {
		m.Tag = v.GetStringValue()
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
