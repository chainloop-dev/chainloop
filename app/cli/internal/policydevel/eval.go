// Copyright 2024-2026 The Chainloop Authors.
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

package policydevel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	controlplanev1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	v12 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
)

const (
	enablePrint = true
)

type EvalOptions struct {
	PolicyPath         string
	MaterialKind       string
	Annotations        map[string]string
	MaterialPath       string
	Inputs             map[string]string
	AllowedHostnames   []string
	Debug              bool
	AttestationClient  controlplanev1.AttestationServiceClient
	ControlPlaneConn   *grpc.ClientConn
	ProjectName        string
	ProjectVersionName string
}

type EvalResult struct {
	Violations  []string          `json:"violations"`
	Findings    []json.RawMessage `json:"findings,omitempty"`
	SkipReasons []string          `json:"skip_reasons"`
	Skipped     bool              `json:"skipped"`
}

type EvalSummary struct {
	Result    *EvalResult           `json:"result"`
	DebugInfo *EvalSummaryDebugInfo `json:"debug_info,omitempty"`
}

type EvalSummaryDebugInfo struct {
	Inputs     []json.RawMessage `json:"inputs"`
	RawResults []json.RawMessage `json:"raw_results"`
}

func Evaluate(opts *EvalOptions, logger zerolog.Logger) (*EvalSummary, error) {
	// 1. Create crafting schema
	policies, err := createPolicies(opts.PolicyPath, opts.Inputs)
	if err != nil {
		return nil, err
	}

	// 2. Craft material with annotations
	crafted, err := craftMaterial(opts.MaterialPath, opts.MaterialKind, &logger)
	if err != nil {
		return nil, err
	}
	material := crafted.Material
	mergeAnnotations(material, opts.Annotations, &logger)

	// 3. Verify material against policy. A crafter that transformed the artifact
	// before storing it hands back what it stored, and that is what the policy
	// must be evaluated against — `policy devel eval` has to reproduce what
	// `attestation add` does, or a policy would be developed against input the
	// real run never sees.
	summary, err := verifyMaterial(policies, material, opts.MaterialPath, crafted.EvaluableContent, opts.Debug, opts.AllowedHostnames, opts.AttestationClient, opts.ControlPlaneConn, opts.ProjectName, opts.ProjectVersionName, &logger)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// mergeAnnotations layers the user's --annotation flags on top of the ones the
// crafter produced, rather than replacing them.
//
// The crafter's annotations carry more than metadata: chainloop.material.redacted
// is how a policy learns that secrets were found and stripped out, and it is what
// makes content resolution fail closed rather than fall back to the un-redacted
// file on disk. Dropping it would hide the redaction from the policy and re-open
// the path this exists to close.
//
// The chainloop.* namespace is therefore crafter-owned and not overridable, so
// that a --annotation flag cannot clear the marker. Crafter.stageMaterial
// protects the equivalent invariant on `attestation add` by refusing to override
// annotations that come from the contract.
func mergeAnnotations(material *v12.Attestation_Material, annotations map[string]string, logger *zerolog.Logger) {
	if len(annotations) == 0 {
		return
	}

	// Crafters that do not go through uploadAndCraft (container image, string)
	// leave the map nil.
	if material.Annotations == nil {
		material.Annotations = make(map[string]string, len(annotations))
	}

	for k, v := range annotations {
		if strings.HasPrefix(k, v12.AnnotationPrefix) {
			logger.Info().Str("annotation", k).Msg("reserved annotation namespace, it is set by the crafter and can not be overridden, skipping")
			continue
		}

		material.Annotations[k] = v
	}
}

func createPolicies(policyPath string, inputs map[string]string) (*v1.Policies, error) {
	// Check if the policy path already has a scheme (chainloop://, http://, https://, file://)
	ref := policyPath
	scheme, _ := policies.RefParts(policyPath)
	if scheme == "" {
		// Default to file://
		ref = fmt.Sprintf("file://%s", policyPath)
	}

	return &v1.Policies{
		Materials: []*v1.PolicyAttachment{
			{
				Policy: &v1.PolicyAttachment_Ref{Ref: ref},
				With:   inputs,
			},
		},
		Attestation: nil,
	}, nil
}

func verifyMaterial(pol *v1.Policies, material *v12.Attestation_Material, materialPath string, evaluableContent []byte, debug bool, allowedHostnames []string, attestationClient controlplanev1.AttestationServiceClient, grpcConn *grpc.ClientConn, projectName, projectVersion string, logger *zerolog.Logger) (*EvalSummary, error) {
	var opts []policies.PolicyVerifierOption
	if len(allowedHostnames) > 0 {
		opts = append(opts, policies.WithAllowedHostnames(allowedHostnames...))
	}

	opts = append(opts, policies.WithIncludeRawData(debug))
	opts = append(opts, policies.WithEnablePrint(enablePrint))
	opts = append(opts, policies.WithGRPCConn(grpcConn))
	if projectName != "" || projectVersion != "" {
		opts = append(opts, policies.WithProjectContext(projectName, projectVersion))
	}

	v := policies.NewPolicyVerifier(pol, attestationClient, logger, opts...)
	policyEvs, err := v.VerifyMaterial(context.Background(), material, materialPath,
		policies.WithMaterialContent(evaluableContent))
	if err != nil {
		return nil, err
	}

	if len(policyEvs) == 0 || policyEvs[0] == nil {
		return nil, fmt.Errorf("no execution branch matched, or all of them were ignored, for kind %s", material.MaterialType.String())
	}

	// Only one evaluation expected for a single policy attachment
	policyEv := policyEvs[0]

	summary := &EvalSummary{
		Result: &EvalResult{
			Skipped:     policyEv.GetSkipped(),
			SkipReasons: policyEv.SkipReasons,
			Violations:  make([]string, 0, len(policyEv.Violations)),
		},
	}

	// Split violations into string messages and structured findings.
	// "violations" contains the message strings (what old CLIs see).
	// "findings" contains the full structured data when present.
	marshaler := protojson.MarshalOptions{UseProtoNames: true}
	for _, v := range policyEv.Violations {
		summary.Result.Violations = append(summary.Result.Violations, v.GetMessage())

		if f := v.GetFinding(); f != nil {
			// Clone to clear subject before marshaling
			vc := proto.Clone(v).(*v12.PolicyEvaluation_Violation)
			vc.Subject = ""
			vc.Message = ""

			b, err := marshaler.Marshal(vc)
			if err != nil {
				return nil, fmt.Errorf("marshaling finding: %w", err)
			}
			summary.Result.Findings = append(summary.Result.Findings, b)
		}
	}

	// Include raw debug info if requested
	if debug {
		summary.DebugInfo = &EvalSummaryDebugInfo{
			Inputs:     []json.RawMessage{},
			RawResults: []json.RawMessage{},
		}

		for _, rr := range policyEv.RawResults {
			if rr == nil {
				continue
			}
			// Take the first input found, as we only allow one material input
			if len(summary.DebugInfo.Inputs) == 0 && rr.Input != nil {
				summary.DebugInfo.Inputs = append(summary.DebugInfo.Inputs, json.RawMessage(rr.Input))
			}
			// Collect all output raw results
			if rr.Output != nil {
				summary.DebugInfo.RawResults = append(summary.DebugInfo.RawResults, json.RawMessage(rr.Output))
			}
		}
	}

	return summary, nil
}

func craftMaterial(materialPath, materialKind string, logger *zerolog.Logger) (*materials.CraftResult, error) {
	backend := &casclient.CASBackend{
		Name:     "backend",
		MaxSize:  0,
		Uploader: nil, // Skip uploads
	}

	// Explicit kind
	if materialKind != "" {
		kind, ok := v1.CraftingSchema_Material_MaterialType_value[materialKind]
		if !ok {
			return nil, fmt.Errorf("invalid material kind: %s", materialKind)
		}
		return craft(materialPath, v1.CraftingSchema_Material_MaterialType(kind), "material", backend, logger)
	}

	// Auto-detect kind
	for _, kind := range v1.CraftingMaterialInValidationOrder {
		m, err := craft(materialPath, kind, "auto-detected-material", backend, logger)
		if err == nil {
			return m, nil
		}
	}

	return nil, fmt.Errorf("could not auto-detect material kind for: %s", materialPath)
}

func craft(materialPath string, kind v1.CraftingSchema_Material_MaterialType, name string, backend *casclient.CASBackend, logger *zerolog.Logger) (*materials.CraftResult, error) {
	materialSchema := &v1.CraftingSchema_Material{
		Type: kind,
		Name: name,
	}

	res, err := materials.Craft(context.Background(), materialSchema, materialPath, backend, nil, logger, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to craft material (kind=%s): %w", kind.String(), err)
	}
	return res, nil
}
