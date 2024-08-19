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

package policies

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	v13 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	v12 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
)

type PolicyError struct {
	err error
}

func NewPolicyError(err error) *PolicyError {
	return &PolicyError{err: err}
}

func (e *PolicyError) Error() string {
	return fmt.Sprintf("policy error: %s", e.err.Error())
}

func (e *PolicyError) Unwrap() error {
	return e.err
}

type PolicyVerifier struct {
	schema *v1.CraftingSchema
	logger *zerolog.Logger
	client v13.AttestationServiceClient
}

func NewPolicyVerifier(schema *v1.CraftingSchema, client v13.AttestationServiceClient, logger *zerolog.Logger) *PolicyVerifier {
	return &PolicyVerifier{schema: schema, client: client, logger: logger}
}

// VerifyMaterial applies all required policies to a material
func (pv *PolicyVerifier) VerifyMaterial(ctx context.Context, material *v12.Attestation_Material, artifactPath string) ([]*v12.PolicyEvaluation, error) {
	result := make([]*v12.PolicyEvaluation, 0)

	policies, err := pv.requiredPoliciesForMaterial(ctx, material)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	for _, policy := range policies {
		// 1. load the policy spec
		spec, err := pv.loadPolicySpec(ctx, policy)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// load the policy script (rego)
		script, err := LoadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// Load material content
		subject, err := getMaterialContent(material, artifactPath)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		pv.logger.Info().Msgf("evaluating policy '%s' against material '%s'", spec.Metadata.Name, material.GetArtifact().GetId())

		// verify the policy
		ng := getPolicyEngine(spec)
		violations, err := ng.Verify(ctx, script, subject, getInputArguments(policy.GetWith()))
		if err != nil {
			return nil, NewPolicyError(err)
		}

		result = append(result, &v12.PolicyEvaluation{
			Name:         spec.GetMetadata().GetName(),
			MaterialName: material.GetArtifact().GetId(),
			Body:         base64.StdEncoding.EncodeToString(script.Source),
			Violations:   engineViolationsToAPIViolations(violations),
			Annotations:  spec.GetMetadata().GetAnnotations(),
			Description:  spec.GetMetadata().GetDescription(),
			With:         policy.GetWith(),
		})
	}

	return result, nil
}

// VerifyStatement verifies that the statement is compliant with the policies present in the schema
func (pv *PolicyVerifier) VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*v12.PolicyEvaluation, error) {
	result := make([]*v12.PolicyEvaluation, 0)
	policies := pv.schema.GetPolicies().GetAttestation()
	for _, policyAtt := range policies {
		// 1. load the policy spec
		spec, err := pv.loadPolicySpec(ctx, policyAtt)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// it's expected statements can only be validated by policy of type ATTESTATION
		if spec.GetSpec().GetType() != v1.CraftingSchema_Material_ATTESTATION {
			continue
		}

		// 2. load the policy script (rego)
		script, err := LoadPolicyScriptFromSpec(spec)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		pv.logger.Info().Msgf("evaluating policy '%s' on attestation", spec.Metadata.Name)

		material, err := protojson.Marshal(statement)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// 4. verify the policy
		ng := getPolicyEngine(spec)
		res, err := ng.Verify(ctx, script, material, getInputArguments(policyAtt.GetWith()))
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// 5. Store result in the attestation itself (for the renderer to include them in the predicate)
		result = append(result, &v12.PolicyEvaluation{
			Name:        spec.Metadata.Name,
			Body:        base64.StdEncoding.EncodeToString(script.Source),
			Violations:  policyViolationsToAttestationViolations(res),
			Annotations: spec.GetMetadata().GetAnnotations(),
			Description: spec.GetMetadata().GetDescription(),
			With:        policyAtt.GetWith(),
		})
	}

	return result, nil
}

// LoadPolicySpec loads and validates a policy spec from a contract
func (pv *PolicyVerifier) loadPolicySpec(ctx context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, error) {
	loader, err := pv.getLoader(attachment)
	if err != nil {
		return nil, fmt.Errorf("failed to get a loader for policy: %w", err)
	}

	spec, err := loader.Load(ctx, attachment)
	if err != nil {
		return nil, fmt.Errorf("loading policy spec: %w", err)
	}

	// Validate just in case
	if err = validatePolicy(spec); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	return spec, nil
}

func (pv *PolicyVerifier) getLoader(attachment *v1.PolicyAttachment) (Loader, error) {
	ref := attachment.GetRef()
	emb := attachment.GetEmbedded()

	if emb == nil && ref == "" {
		return nil, errors.New("policy must be referenced or embedded in the attachment")
	}

	// Figure out loader to use
	var loader Loader
	if emb != nil {
		loader = new(EmbeddedLoader)
	} else {
		if IsProviderScheme(ref) {
			loader = NewChainloopLoader(pv.client)
		} else {
			loader = new(BlobLoader)
		}
		pv.logger.Debug().Msgf("loading policy spec %q using %T", ref, loader)
	}

	return loader, nil
}

func validatePolicy(policy *v1.Policy) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("validating policy spec: %w", err)
	}
	err = validator.Validate(policy)
	if err != nil {
		return fmt.Errorf("validating policy spec: %w", err)
	}

	return nil
}

func getInputArguments(inputs map[string]string) map[string]any {
	args := make(map[string]any)
	for k, v := range inputs {
		// scan for multiple values
		lines := strings.Split(strings.TrimRight(v, "\n"), "\n")
		value := getValue(lines)

		if value == nil {
			continue
		}
		s, ok := value.(string)
		if !ok {
			// case for multivalued argument
			args[k] = value
		}

		// Single string, let's check for CSV
		lines = strings.Split(s, ",")
		value = getValue(lines)
		if value == nil {
			continue
		}
		args[k] = value
	}

	return args
}

func getValue(values []string) any {
	lines := make([]string, 0)
	for _, line := range values {
		text := strings.TrimSpace(line)
		if len(text) > 0 {
			lines = append(lines, text)
		}
	}

	if len(lines) == 0 {
		// No valid input, skip
		return nil
	}
	if len(lines) > 1 {
		return lines
	}
	// nolint: gosec
	return lines[0]
}

func engineViolationsToAPIViolations(input []*engine.PolicyViolation) []*v12.PolicyEvaluation_Violation {
	res := make([]*v12.PolicyEvaluation_Violation, 0)
	for _, v := range input {
		res = append(res, &v12.PolicyEvaluation_Violation{
			Subject: v.Subject,
			Message: v.Violation,
		})
	}

	return res
}

func getMaterialContent(material *v12.Attestation_Material, artifactPath string) ([]byte, error) {
	var rawMaterial []byte
	var err error

	// nolint: gocritic
	if material.InlineCas {
		rawMaterial = material.GetArtifact().GetContent()
	} else if artifactPath == "" {
		return nil, errors.New("artifact path required")
	} else {
		// read content from local filesystem
		rawMaterial, err = os.ReadFile(artifactPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read material content: %w", err)
		}
	}
	// special case for ATTESTATION materials, the statement needs to be extracted from the dsse wrapper.
	if material.MaterialType == v1.CraftingSchema_Material_ATTESTATION {
		var envelope dsse.Envelope
		if err := json.Unmarshal(rawMaterial, &envelope); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attestation material: %w", err)
		}

		rawMaterial, err = envelope.DecodeB64Payload()
		if err != nil {
			return nil, fmt.Errorf("failed to decode attestation material: %w", err)
		}
	}

	return rawMaterial, nil
}

// returns the list of polices to be applied to a material, following these rules:
// 1. if policy spec has a type, return it only if material has the same type
// 2. if attachment has a name filter, return the policy only if the material has the same name
// 3. if policy spec doesn't have a type, a name filter is mandatory (otherwise there is no way to know if material has to be applied)
func (pv *PolicyVerifier) requiredPoliciesForMaterial(ctx context.Context, material *v12.Attestation_Material) ([]*v1.PolicyAttachment, error) {
	result := make([]*v1.PolicyAttachment, 0)
	policies := pv.schema.GetPolicies().GetMaterials()

	for _, policyAtt := range policies {
		// load the policy spec
		spec, err := pv.loadPolicySpec(ctx, policyAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy spec: %w", err)
		}

		specType := spec.GetSpec().GetType()
		materialType := material.GetMaterialType()
		filteredName := policyAtt.GetSelector().GetName()

		// if spec has a type, and it's different to the material type, skip
		if specType != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && specType != materialType {
			// types don't match, continue
			continue
		}

		if filteredName != "" && filteredName != material.GetArtifact().GetId() {
			// a filer exists and doesn't match
			continue
		}

		// no type nor name to match, we can't guess anything
		if specType == v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED && filteredName == "" {
			continue
		}

		result = append(result, policyAtt)
	}

	return result, nil
}

// getPolicyEngine returns a PolicyEngine implementation to evaluate a given policy.
func getPolicyEngine(_ *v1.Policy) engine.PolicyEngine {
	// Currently, only Rego is supported
	return new(rego.Rego)
}

func policyViolationsToAttestationViolations(violations []*engine.PolicyViolation) (pvs []*v12.PolicyEvaluation_Violation) {
	for _, violation := range violations {
		pvs = append(pvs, &v12.PolicyEvaluation_Violation{
			Subject: violation.Subject,
			Message: violation.Violation,
		})
	}
	return
}

// LoadPolicyScriptFromSpec loads a policy referenced from the spec
func LoadPolicyScriptFromSpec(spec *v1.Policy) (*engine.Policy, error) {
	var content []byte
	var err error

	switch source := spec.GetSpec().GetSource().(type) {
	case *v1.PolicySpec_Embedded:
		content = []byte(source.Embedded)
	case *v1.PolicySpec_Path:
		content, err = blob.LoadFileOrURL(source.Path)
		if err != nil {
			return nil, fmt.Errorf("loading policy content: %w", err)
		}
	default:
		return nil, fmt.Errorf("policy spec is empty")
	}

	return &engine.Policy{
		Name:   spec.GetMetadata().GetName(),
		Source: content,
	}, nil
}

func LogPolicyViolations(evaluations []*v12.PolicyEvaluation, logger *zerolog.Logger) {
	for _, policyEval := range evaluations {
		if len(policyEval.Violations) > 0 {
			subject := policyEval.MaterialName
			if subject == "" {
				subject = "statement"
			}
			logger.Warn().Msgf("found policy violations (%s) for %s", policyEval.Name, subject)
			for _, v := range policyEval.Violations {
				logger.Warn().Msgf(" - %s", v.Message)
			}
		}
	}
}
