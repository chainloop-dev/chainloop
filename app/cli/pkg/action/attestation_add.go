//
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

package action

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"google.golang.org/grpc"
)

type AttestationAddOpts struct {
	*ActionsOpts
	ArtifactsCASConn   *grpc.ClientConn
	CASURI             string
	CASCAPath          string // optional CA certificate for the CAS connection
	ConnectionInsecure bool
	// OCI registry credentials used for CONTAINER_IMAGE material type
	RegistryServer, RegistryUsername, RegistryPassword string
	LocalStatePath                                     string
	// NoStrictValidation skips strict schema validation
	NoStrictValidation bool
}

type newCrafterOpts struct {
	cpConnection *grpc.ClientConn
	opts         []crafter.NewOpt
}

type AttestationAdd struct {
	*ActionsOpts
	casURI string
	// optional CA certificate for the CAS connection
	casCAPath          string
	connectionInsecure bool
	localStatePath     string
	*newCrafterOpts
}

func NewAttestationAdd(cfg *AttestationAddOpts) (*AttestationAdd, error) {
	opts := []crafter.NewOpt{crafter.WithLogger(&cfg.Logger)}
	if cfg.RegistryServer != "" && cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		cfg.Logger.Debug().Str("server", cfg.RegistryServer).Str("username", cfg.RegistryUsername).Msg("using OCI registry credentials")
		opts = append(opts, crafter.WithOCIAuth(cfg.RegistryServer, cfg.RegistryUsername, cfg.RegistryPassword))
	}
	if cfg.NoStrictValidation {
		opts = append(opts, crafter.WithNoStrictValidation(cfg.NoStrictValidation))
	}

	return &AttestationAdd{
		ActionsOpts:        cfg.ActionsOpts,
		newCrafterOpts:     &newCrafterOpts{cpConnection: cfg.CPConnection, opts: opts},
		casURI:             cfg.CASURI,
		casCAPath:          cfg.CASCAPath,
		connectionInsecure: cfg.ConnectionInsecure,
		localStatePath:     cfg.LocalStatePath,
	}, nil
}

var ErrAttestationNotInitialized = errors.New("attestation not yet initialized")

func (action *AttestationAdd) Run(ctx context.Context, attestationID, materialName, materialValue, materialType string, annotations map[string]string, policyInputFiles []*PolicyInputFromFile) (*AttestationStatusMaterial, error) {
	// initialize the crafter. If attestation-id is provided we assume the attestation is performed using remote state
	crafter, err := newCrafter(&newCrafterStateOpts{enableRemoteState: (attestationID != ""), localStatePath: action.localStatePath}, action.CPConnection, action.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	// Resolve runtime policy inputs from the provided files before adding the
	// material, so a malformed file aborts the add early.
	runtimeInputs, err := buildRuntimeInputs(policyInputFiles)
	if err != nil {
		return nil, err
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	// Default to inline CASBackend and override if we are not in dry-run mode
	casBackend := &casclient.CASBackend{
		Name: "not-set",
	}

	// Define CASbackend information based on the API response
	if !crafter.CraftingState.GetDryRun() {
		client := pb.NewAttestationServiceClient(action.CPConnection)
		workflowRunID := crafter.CraftingState.GetAttestation().GetWorkflow().GetWorkflowRunId()
		_, connectionCloserFn, getCASBackendErr := getCASBackend(ctx, client, workflowRunID, action.casCAPath, action.casURI, action.connectionInsecure, action.Logger, casBackend, action.CLIVersion)
		if getCASBackendErr != nil {
			return nil, fmt.Errorf("failed to get CAS backend: %w", getCASBackendErr)
		}
		if connectionCloserFn != nil {
			// nolint: errcheck
			defer connectionCloserFn()
		}
	}

	// Add material to the attestation crafting state based on if the material is contract free or not.
	// The checks are performed in the following order:
	// 1. If materialName is empty and materialType is empty, we don't know anything about the material so, we add it with auto-detected kind and random name
	// 2. If materialName is not empty, check if the material is in the contract. If it is, add material from contract
	// 2.1. If materialType is empty, try to guess the material kind with auto-detected kind and materialName
	// 3. If materialType is not empty, add material contract free with materialType and materialName
	addOpts := runtimeInputAddOpts(runtimeInputs)

	var mt *api.Attestation_Material
	switch {
	case materialName == "" && materialType == "":
		mt, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, "", materialValue, casBackend, annotations, addOpts...)
		if err != nil {
			return nil, fmt.Errorf("adding material: %w", err)
		}
		action.Logger.Info().Str("kind", mt.MaterialType.String()).Msg("material kind detected")
	case materialName != "":
		switch {
		// If the material is in the contract, add it from the contract
		case crafter.IsMaterialInContract(materialName):
			mt, err = crafter.AddMaterialFromContract(ctx, attestationID, materialName, materialValue, casBackend, annotations, addOpts...)
		// If the material is not in the contract and the materialType is not provided, add material contract free with auto-detected kind, guessing the kind
		case materialType == "":
			mt, err = crafter.AddMaterialContactFreeWithAutoDetectedKind(ctx, attestationID, materialName, materialValue, casBackend, annotations, addOpts...)
			if err != nil {
				return nil, fmt.Errorf("adding material: %w", err)
			}
			action.Logger.Info().Str("kind", mt.MaterialType.String()).Msg("material kind detected")
		// If the material is not in the contract and has a materialType, add material contract free with the provided materialType
		default:
			mt, err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations, addOpts...)
		}
	default:
		mt, err = crafter.AddMaterialContractFree(ctx, attestationID, materialType, materialName, materialValue, casBackend, annotations, addOpts...)
	}

	if err != nil {
		return nil, fmt.Errorf("adding material: %w", err)
	}

	// Record each source file as an EVIDENCE material, cross-linked to the
	// evaluated material so the exemption set itself is attested.
	if err := action.addPolicyInputEvidence(ctx, crafter, attestationID, mt, policyInputFiles, casBackend); err != nil {
		return nil, fmt.Errorf("recording policy input evidence: %w", err)
	}

	materialResult, err := attMaterialToAction(mt)
	if err != nil {
		return nil, fmt.Errorf("converting material to action: %w", err)
	}

	return materialResult, nil
}

// runtimeInputAddOpts wraps the runtime inputs as crafter add options, or
// returns nil when there are none. Defined at package scope so it can name the
// crafter package type (the Run method shadows it with a local variable).
func runtimeInputAddOpts(runtimeInputs *policies.RuntimeInputs) []crafter.AddOpt {
	if runtimeInputs == nil || (len(runtimeInputs.Global) == 0 && len(runtimeInputs.Scoped) == 0) {
		return nil
	}
	return []crafter.AddOpt{crafter.WithRuntimeInputs(runtimeInputs)}
}

// buildRuntimeInputs reads each policy input file and returns the extracted
// values grouped for the policy engine: unscoped entries under Global and
// policy-scoped entries under Scoped[policy]. Values are newline-joined and
// accumulated via policies.MergeRuntimeInputs so repeated inputs merge using the
// same multi-value encoding the engine expects (it splits inputs back on
// newlines and commas). As with contract-declared arguments, individual values
// must not embed those delimiters; path globs, the intended use, never do.
func buildRuntimeInputs(policyInputFiles []*PolicyInputFromFile) (*policies.RuntimeInputs, error) {
	if len(policyInputFiles) == 0 {
		return nil, nil
	}

	ri := &policies.RuntimeInputs{
		Global: map[string]string{},
		Scoped: map[string]map[string]string{},
	}
	for _, pif := range policyInputFiles {
		values, err := ExtractColumnValues(pif.File, pif.Column)
		if err != nil {
			return nil, fmt.Errorf("extracting %q from %q: %w", pif.Column, pif.File, err)
		}

		// Unscoped entries go to Global; policy-scoped entries to their own
		// Scoped[policy] map. Because global and scoped values live in separate
		// maps, they never collide here even when they share an input name;
		// forPolicy is what later merges a policy's scoped values over Global.
		add := map[string]string{pif.Input: strings.Join(values, "\n")}
		if pif.Policy == "" {
			ri.Global = policies.MergeRuntimeInputs(ri.Global, add)
		} else {
			ri.Scoped[pif.Policy] = policies.MergeRuntimeInputs(ri.Scoped[pif.Policy], add)
		}
	}

	return ri, nil
}

// addPolicyInputEvidence adds each policy input file as an EVIDENCE material,
// cross-linked with the evaluated material in both directions via the
// chainloop.material.references annotation: each evidence material points at the
// subject, and the subject points back at all of its evidence materials. The
// exact policy input the file fed is recorded in the chainloop.material.policy_input
// annotation (the material name carries only a sanitized form of it).
func (action *AttestationAdd) addPolicyInputEvidence(ctx context.Context, c *crafter.Crafter, attestationID string, subject *api.Attestation_Material, policyInputFiles []*PolicyInputFromFile, casBackend *casclient.CASBackend) error {
	names := policyInputEvidenceNames(subject.GetId(), policyInputFiles)

	// Reverse edge: point the subject back at its evidence materials. The
	// subject is the same in-memory object held in the crafting state, so this
	// is persisted when the evidence materials below are written.
	addReference(subject, names...)

	for i, pif := range policyInputFiles {
		annotations := map[string]string{
			materials.AnnotationMaterialReferences: subject.GetId(),
			materials.AnnotationPolicyInput:        pif.Input,
		}

		if _, err := c.AddMaterialContractFree(ctx, attestationID, schemaapi.CraftingSchema_Material_EVIDENCE.String(), names[i], pif.File, casBackend, annotations); err != nil {
			return fmt.Errorf("adding evidence material %q: %w", names[i], err)
		}
	}

	return nil
}

// addReference appends the given material names to m's chainloop.material.references
// annotation (comma-separated), preserving any existing references and skipping
// duplicates.
func addReference(m *api.Attestation_Material, names ...string) {
	if len(names) == 0 {
		return
	}
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}

	existing := []string{}
	if v := m.Annotations[materials.AnnotationMaterialReferences]; v != "" {
		existing = strings.Split(v, ",")
	}

	seen := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		seen[e] = struct{}{}
	}
	for _, n := range names {
		if _, ok := seen[n]; !ok {
			existing = append(existing, n)
			seen[n] = struct{}{}
		}
	}

	m.Annotations[materials.AnnotationMaterialReferences] = strings.Join(existing, ",")
}

// policyInputEvidenceNames derives the evidence material name for each policy
// input file, in order, as "<material>-<input>". The input name is sanitized to
// a valid material-name part (lowercase; runs of characters outside [a-z0-9]
// collapsed to a single "-"), since material names must match
// ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ and input names may contain e.g. underscores
// ("ignored_paths" -> "ignored-paths"). When two files map to the same
// sanitized name, a "-<n>" suffix keeps them unique so no record is
// overwritten. The exact input name is preserved in the
// chainloop.material.policy_input annotation.
func policyInputEvidenceNames(materialName string, policyInputFiles []*PolicyInputFromFile) []string {
	base := make([]string, len(policyInputFiles))
	count := make(map[string]int, len(policyInputFiles))
	for i, pif := range policyInputFiles {
		base[i] = materialName + "-" + sanitizeMaterialNamePart(pif.Input)
		count[base[i]]++
	}

	names := make([]string, len(policyInputFiles))
	seen := make(map[string]int, len(policyInputFiles))
	for i, b := range base {
		if count[b] > 1 {
			seen[b]++
			names[i] = fmt.Sprintf("%s-%d", b, seen[b])
		} else {
			names[i] = b
		}
	}

	return names
}

// sanitizeMaterialNamePart lower-cases s and collapses every run of characters
// outside [a-z0-9] into a single "-", trimming leading/trailing "-", so the
// result is a valid material-name component. Falls back to "input" if nothing
// usable remains.
func sanitizeMaterialNamePart(s string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			pendingHyphen = false
		} else {
			pendingHyphen = true
		}
	}

	if b.Len() == 0 {
		return "input"
	}
	return b.String()
}

// GetPolicyEvaluations is a Wrapper around the getPolicyEvaluations
func (action *AttestationAdd) GetPolicyEvaluations(ctx context.Context, attestationID string) (map[string][]*PolicyEvaluation, error) {
	crafter, err := newCrafter(&newCrafterStateOpts{enableRemoteState: (attestationID != ""), localStatePath: action.localStatePath}, action.CPConnection, action.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	if initialized, err := crafter.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := crafter.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	policyEvaluations, _ := getPolicyEvaluations(crafter)

	return policyEvaluations, nil
}
