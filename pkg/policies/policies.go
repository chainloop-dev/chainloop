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
	"path/filepath"
	"slices"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	v13 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/rs/zerolog"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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
	return e.err.Error()
}

func (e *PolicyError) Unwrap() error {
	return e.err
}

type Verifier interface {
	VerifyMaterial(ctx context.Context, m *v12.Attestation_Material, path string) ([]*v12.PolicyEvaluation, error)
	VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*v12.PolicyEvaluation, error)
}

type PolicyVerifier struct {
	schema *v1.CraftingSchema
	logger *zerolog.Logger
	client v13.AttestationServiceClient
}

var _ Verifier = (*PolicyVerifier)(nil)

func NewPolicyVerifier(schema *v1.CraftingSchema, client v13.AttestationServiceClient, logger *zerolog.Logger) *PolicyVerifier {
	return &PolicyVerifier{schema: schema, client: client, logger: logger}
}

// VerifyMaterial applies all required policies to a material
func (pv *PolicyVerifier) VerifyMaterial(ctx context.Context, material *v12.Attestation_Material, artifactPath string) ([]*v12.PolicyEvaluation, error) {
	result := make([]*v12.PolicyEvaluation, 0)

	attachments, err := pv.requiredPoliciesForMaterial(ctx, material)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	for _, attachment := range attachments {
		// Load material content
		subject, err := getMaterialContent(material, artifactPath)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		ev, err := pv.evaluatePolicyAttachment(ctx, attachment, subject,
			&evalOpts{kind: material.MaterialType, name: material.GetArtifact().GetId()},
		)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		result = append(result, ev)
	}

	return result, nil
}

type evalOpts struct {
	name string
	kind v1.CraftingSchema_Material_MaterialType
}

func (pv *PolicyVerifier) evaluatePolicyAttachment(ctx context.Context, attachment *v1.PolicyAttachment, material []byte, opts *evalOpts) (*v12.PolicyEvaluation, error) {
	// 1. load the policy policy
	policy, ref, err := pv.loadPolicySpec(ctx, attachment)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	var basePath string
	// if it's a file://, let's calculate the base path for loading referenced policies, from the loader ref
	if ref != nil {
		// calculate the file path if it's a file:// reference
		basePath, _ = ensureScheme(attachment.GetRef(), fileScheme)
	}

	// load the policy scripts (rego)
	scripts, err := LoadPolicyScriptsFromSpec(policy, opts.kind, basePath)
	if err != nil {
		return nil, NewPolicyError(err)
	}

	if opts.name != "" {
		pv.logger.Info().Msgf("evaluating policy %s against %s", policy.Metadata.Name, opts.name)
	} else {
		pv.logger.Info().Msgf("evaluating policy %s against attestation", policy.Metadata.Name)
	}

	sources := make([]string, 0)
	evalResults := make([]*engine.EvaluationResult, 0)
	skipped := true
	reasons := make([]string, 0)
	for _, script := range scripts {
		r, err := pv.executeScript(ctx, policy, script, material, attachment)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		// Gather merged results
		evalResults = append(evalResults, r)

		if r.SkipReason != "" {
			reasons = append(reasons, r.SkipReason)
		}

		// Skipped = false if any of the evaluations was not skipped
		skipped = skipped && r.Skipped
		sources = append(sources, base64.StdEncoding.EncodeToString(script.Source))
	}

	var evaluationSources []string
	if !IsProviderScheme(ref.GetName()) {
		evaluationSources = sources
	}

	// Only inform skip reasons if it's skipped
	if !skipped {
		reasons = []string{}
	}

	// Merge multi-kind results
	return &v12.PolicyEvaluation{
		Name:         policy.GetMetadata().GetName(),
		MaterialName: opts.name,
		Sources:      evaluationSources,
		// merge all violations
		Violations:      engineEvaluationsToAPIViolations(evalResults),
		Annotations:     policy.GetMetadata().GetAnnotations(),
		Description:     policy.GetMetadata().GetDescription(),
		With:            attachment.GetWith(),
		Type:            opts.kind,
		ReferenceName:   ref.Name,
		ReferenceDigest: ref.Digest["sha256"],
		// Merged "skipped"
		Skipped: skipped,
		// Merged "skip_reason"
		SkipReasons: reasons,
	}, nil
}

// VerifyStatement verifies that the statement is compliant with the policies present in the schema
func (pv *PolicyVerifier) VerifyStatement(ctx context.Context, statement *intoto.Statement) ([]*v12.PolicyEvaluation, error) {
	result := make([]*v12.PolicyEvaluation, 0)
	policies := pv.schema.GetPolicies().GetAttestation()
	for _, policyAtt := range policies {
		material, err := protojson.Marshal(statement)
		if err != nil {
			return nil, NewPolicyError(err)
		}

		ev, err := pv.evaluatePolicyAttachment(ctx, policyAtt, material, &evalOpts{kind: v1.CraftingSchema_Material_ATTESTATION})
		if err != nil {
			return nil, NewPolicyError(err)
		}

		result = append(result, ev)
	}

	return result, nil
}

func (pv *PolicyVerifier) executeScript(ctx context.Context, policy *v1.Policy, script *engine.Policy, material []byte, att *v1.PolicyAttachment) (*engine.EvaluationResult, error) {
	// verify the policy
	ng := getPolicyEngine(policy)
	res, err := ng.Verify(ctx, script, material, getInputArguments(att.GetWith()))
	if err != nil {
		return nil, fmt.Errorf("failed to execute policy : %w", err)
	}

	return res, nil
}

// LoadPolicySpec loads and validates a policy spec from a contract
func (pv *PolicyVerifier) loadPolicySpec(ctx context.Context, attachment *v1.PolicyAttachment) (*v1.Policy, *v12.ResourceDescriptor, error) {
	loader, err := pv.getLoader(attachment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get a loader for policy: %w", err)
	}

	spec, ref, err := loader.Load(ctx, attachment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load policy spec: %w", err)
	}

	// Validate just in case
	if err = validateResource(spec); err != nil {
		return nil, nil, err
	}

	return spec, ref, nil
}

func (pv *PolicyVerifier) getLoader(attachment *v1.PolicyAttachment) (Loader, error) {
	ref := attachment.GetRef()
	emb := attachment.GetEmbedded()

	if emb == nil && ref == "" {
		return nil, errors.New("policy must be referenced or embedded in the attachment")
	}

	// Figure out loader to use
	if emb != nil {
		return new(EmbeddedLoader), nil
	}

	var loader Loader
	scheme, _ := refParts(ref)
	switch scheme {
	// No scheme means chainloop loader
	case chainloopScheme, "":
		loader = NewChainloopLoader(pv.client)
	case fileScheme:
		loader = new(FileLoader)
	case httpsScheme, httpScheme:
		loader = new(HTTPSLoader)
	default:
		return nil, fmt.Errorf("policy scheme not supported: %s", scheme)
	}

	pv.logger.Debug().Msgf("loading policy spec %q using %T", ref, loader)

	return loader, nil
}

func validateResource(m proto.Message) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("validating policy spec: %w", err)
	}
	err = validator.Validate(m)
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

func engineEvaluationsToAPIViolations(results []*engine.EvaluationResult) []*v12.PolicyEvaluation_Violation {
	res := make([]*v12.PolicyEvaluation_Violation, 0)
	for _, r := range results {
		for _, v := range r.Violations {
			res = append(res, &v12.PolicyEvaluation_Violation{
				Subject: v.Subject,
				Message: v.Violation,
			})
		}
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
		apply, err := pv.shouldApplyPolicy(ctx, policyAtt, material)
		if err != nil {
			return nil, err
		}

		if apply {
			result = append(result, policyAtt)
		}
	}

	return result, nil
}

func (pv *PolicyVerifier) shouldApplyPolicy(ctx context.Context, policyAtt *v1.PolicyAttachment, material *v12.Attestation_Material) (bool, error) {
	// load the policy spec
	spec, _, err := pv.loadPolicySpec(ctx, policyAtt)
	if err != nil {
		return false, fmt.Errorf("failed to load policy attachment %q: %w", policyAtt.GetRef(), err)
	}

	materialType := material.GetMaterialType()
	filteredName := policyAtt.GetSelector().GetName()
	specTypes := getPolicyTypes(spec)

	// if spec has a type, and it's different to the material type, skip
	if len(specTypes) > 0 && !slices.Contains(specTypes, materialType) {
		// types don't match, continue
		return false, nil
	}

	if filteredName != "" && filteredName != material.GetArtifact().GetId() {
		// a filer exists and doesn't match
		return false, nil
	}

	// no type nor name to match, we can't guess anything
	if len(specTypes) == 0 && filteredName == "" {
		return false, nil
	}

	return true, nil
}

func getPolicyTypes(p *v1.Policy) []v1.CraftingSchema_Material_MaterialType {
	policyTypes := make([]v1.CraftingSchema_Material_MaterialType, 0)
	v1Type := p.GetSpec().GetType()
	if v1Type != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED {
		policyTypes = append(policyTypes, v1Type)
	} else {
		for _, branch := range p.GetSpec().GetPolicies() {
			if branch.GetKind() != v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED {
				policyTypes = append(policyTypes, branch.GetKind())
			}
		}
	}
	return policyTypes
}

// getPolicyEngine returns a PolicyEngine implementation to evaluate a given policy.
func getPolicyEngine(_ *v1.Policy) engine.PolicyEngine {
	// Currently, only Rego is supported
	return &rego.Rego{
		// Set the default operating mode to restrictive
		OperatingMode: rego.EnvironmentModeRestrictive,
	}
}

// LoadPolicyScriptsFromSpec loads all policy script that matches a given material type. It matches if:
// * the policy kind is unspecified, meaning that it was forced by name selector
// * the policy kind is specified, and it's equal to the material type
func LoadPolicyScriptsFromSpec(policy *v1.Policy, kind v1.CraftingSchema_Material_MaterialType, basePath string) ([]*engine.Policy, error) {
	scripts := make([]*engine.Policy, 0)

	if policy.GetSpec().GetSource() != nil {
		script, err := loadLegacyPolicyScript(policy.GetSpec(), basePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy script: %w", err)
		}
		scripts = append(scripts, &engine.Policy{Source: script, Name: policy.GetMetadata().GetName()})
	} else {
		// multi-kind policies
		specs := policy.GetSpec().GetPolicies()
		for _, spec := range specs {
			if spec.GetKind() == v1.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED || spec.GetKind() == kind {
				script, err := loadPolicyScript(spec, basePath)
				if err != nil {
					return nil, fmt.Errorf("failed to load policy script: %w", err)
				}
				scripts = append(scripts, &engine.Policy{Source: script, Name: policy.GetMetadata().GetName()})
			}
		}
	}

	return scripts, nil
}

func loadPolicyScript(spec *v1.PolicySpecV2, basePath string) ([]byte, error) {
	var content []byte
	var err error
	switch source := spec.GetSource().(type) {
	case *v1.PolicySpecV2_Embedded:
		content = []byte(source.Embedded)
	case *v1.PolicySpecV2_Path:
		// path relative to policy folder
		scriptPath := filepath.Join(filepath.Dir(basePath), source.Path)
		content, err = blob.LoadFileOrURL(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("loading policy content: %w", err)
		}
	default:
		return nil, fmt.Errorf("policy spec is empty")
	}

	return content, nil
}

func loadLegacyPolicyScript(spec *v1.PolicySpec, basePath string) ([]byte, error) {
	// legacy policies
	var content []byte
	var err error
	switch source := spec.GetSource().(type) {
	case *v1.PolicySpec_Embedded:
		content = []byte(source.Embedded)
	case *v1.PolicySpec_Path:
		// path relative to policy folder
		scriptPath := filepath.Join(filepath.Dir(basePath), source.Path)
		content, err = blob.LoadFileOrURL(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("loading policy content: %w", err)
		}
	default:
		return nil, fmt.Errorf("policy spec is empty")
	}

	return content, nil
}

func LogPolicyEvaluations(evaluations []*v12.PolicyEvaluation, logger *zerolog.Logger) {
	for _, policyEval := range evaluations {
		subject := policyEval.MaterialName
		if subject == "" {
			subject = "statement"
		}

		if policyEval.Skipped {
			logger.Warn().Msgf("policy evaluation skipped (%s) for %s. Reasons: %s", policyEval.Name, subject, policyEval.SkipReasons)
		}
		if len(policyEval.Violations) > 0 {
			logger.Warn().Msgf("found policy violations (%s) for %s", policyEval.Name, subject)
			for _, v := range policyEval.Violations {
				logger.Warn().Msgf(" - %s", v.Message)
			}
		}
	}
}
