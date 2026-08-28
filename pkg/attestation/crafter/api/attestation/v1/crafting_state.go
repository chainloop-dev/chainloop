//
// Copyright 2023-2026 The Chainloop Authors.
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

package v1

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/accesschk"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/cobertura"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/dranzer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/jacoco"
	materialsjunit "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/junit"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/pitest"
	materialsradamsa "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/radamsa"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/trufflehog"
	"github.com/chainloop-dev/chainloop/pkg/tabular"
	intoto "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const AnnotationPrefix = "chainloop."

// AnnotationValueTrue is the value of an annotation that acts as a flag. Shared
// so that whoever sets one and whoever reads it cannot drift apart.
const AnnotationValueTrue = "true"

var (
	AnnotationMaterialType              = CreateAnnotation("material.type")
	AnnotationMaterialName              = CreateAnnotation("material.name")
	AnnotationMaterialSignature         = CreateAnnotation("material.signature")
	AnnotationSignatureDigest           = CreateAnnotation("material.signature.digest")
	AnnotationSignatureProvider         = CreateAnnotation("material.signature.provider")
	AnnotationMaterialCAS               = CreateAnnotation("material.cas")
	AnnotationMaterialInlineCAS         = CreateAnnotation("material.cas.inline")
	AnnotationContainerTag              = CreateAnnotation("material.image.tag")
	AnnotationsContainerLatestTag       = CreateAnnotation("material.image.is_latest_tag")
	AnnotationsSBOMMainComponentName    = CreateAnnotation("material.sbom.main_component.name")
	AnnotationsSBOMMainComponentType    = CreateAnnotation("material.sbom.main_component.type")
	AnnotationsSBOMMainComponentVersion = CreateAnnotation("material.sbom.main_component.version")

	// AnnotationMaterialRedacted marks a material whose stored content was
	// transformed by its crafter before upload, to strip secrets out of it. Two
	// things follow from it: the recorded digest describes the redacted artifact
	// rather than the file on disk, and policy evaluation must be handed that
	// sanitized copy explicitly, because the file on disk still holds the secrets
	// (see GetEvaluableContentFrom, which fails closed without it).
	AnnotationMaterialRedacted = CreateAnnotation("material.redacted")
	// AnnotationMaterialRedactionCount is how many secrets were replaced.
	AnnotationMaterialRedactionCount = CreateAnnotation("material.redaction.count")
	// AnnotationMaterialRedactionRules lists the detection rules that matched,
	// so a policy can act on the kind of credential that was present.
	AnnotationMaterialRedactionRules = CreateAnnotation("material.redaction.rules")
	// AnnotationMaterialRedactionSkipped marks a material uploaded without
	// redaction because the operator explicitly asked for it. Recorded so the
	// bypass is visible to policies rather than silent.
	AnnotationMaterialRedactionSkipped = CreateAnnotation("material.redaction.skipped")
)

type NormalizedMaterialOutput struct {
	Name, Digest string
	IsOutput     bool
	Content      []byte
}

// NormalizedOutput returns a common representation of the properties of a material
// regardless of how it's been encoded.
// For example, it's common to have materials based on artifacts, so we want to normalize the output
func (m *Attestation_Material) NormalizedOutput() (*NormalizedMaterialOutput, error) {
	if m == nil {
		return nil, errors.New("material not provided")
	}

	if a := m.GetContainerImage(); a != nil {
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, nil}, nil
	}

	if a := m.GetString_(); a != nil {
		return &NormalizedMaterialOutput{Content: []byte(a.Value), Digest: a.GetDigest()}, nil
	}

	if a := m.GetArtifact(); a != nil {
		return &NormalizedMaterialOutput{a.Name, a.Digest, a.IsSubject, a.Content}, nil
	}

	if a := m.GetSbomArtifact(); a != nil {
		ar := a.GetArtifact()
		return &NormalizedMaterialOutput{ar.Name, ar.Digest, ar.IsSubject, ar.Content}, nil
	}

	return nil, fmt.Errorf("unknown material: %s", m.MaterialType)
}

// ErrRedactedContentRequired is returned when policy evaluation is asked to
// resolve a material whose stored copy was sanitized, without being given that
// copy. There is no safe source left: the file on disk is the un-redacted
// original, and for a CAS-backed material the sanitized bytes were streamed away
// and are not on the material at all.
var ErrRedactedContentRequired = errors.New(
	"sanitized content is required to evaluate a redacted material: " +
		"policies must never be handed the un-redacted original")

// GetEvaluableContent returns the content to be sent to policy evaluations,
// resolved from the material's stored copy or the file on disk.
func (m *Attestation_Material) GetEvaluableContent(value string) ([]byte, error) {
	return m.GetEvaluableContentFrom(value, nil)
}

// GetEvaluableContentFrom is GetEvaluableContent with an explicit content source.
//
// content, when non-empty, is what policies are evaluated against, overriding
// both the inline bytes and the file on disk. Crafters that transform an artifact
// before it leaves the machine — redacting secrets out of an AI coding session —
// hand back the bytes they stored so that the policy engine sees exactly those,
// whatever CAS backend is in use.
//
// A material marked redacted with no content supplied fails closed. One
// consequence is worth knowing: such a material's policy input cannot be
// reconstructed from persisted crafting state alone, so any future push-time or
// server-side material evaluation has to plumb the bytes through as well.
func (m *Attestation_Material) GetEvaluableContentFrom(value string, content []byte) ([]byte, error) {
	var rawMaterial []byte
	var err error

	// Fail closed before any source is chosen: a material whose stored copy was
	// sanitized has exactly one valid policy input, and it is not the file on
	// disk. Checked here rather than inside the artifact branch below so that it
	// also covers material kinds that carry no artifact.
	if len(content) == 0 && m.GetAnnotations()[AnnotationMaterialRedacted] == AnnotationValueTrue {
		return nil, ErrRedactedContentRequired
	}

	artifact := m.GetArtifact()
	if artifact == nil && m.GetSbomArtifact() != nil {
		artifact = m.GetSbomArtifact().GetArtifact()
	}

	if artifact != nil {
		switch {
		case len(content) > 0:
			// NOTE: ingestMaterialToJSON re-reads the artifact from `value` for
			// the kinds it projects from a path (JUNIT_XML, HELM_CHART), so
			// supplied content would be silently ignored for those. Unreachable
			// today, since only CHAINLOOP_AI_CODING_SESSION supplies content and
			// it is JSON-native. A crafter that starts transforming what it
			// stores for a path-projected kind must make the projection
			// content-based first, or it will fail open.
			rawMaterial = content
		case m.InlineCas:
			rawMaterial = artifact.GetContent()
		case value == "":
			return nil, errors.New("artifact path required")
		case m.MaterialType != v1.CraftingSchema_Material_HELM_CHART &&
			m.MaterialType != v1.CraftingSchema_Material_JUNIT_XML &&
			m.MaterialType != v1.CraftingSchema_Material_RADAMSA_CRASHES:
			// read content from local filesystem (except for tgz charts and
			// metadata-only materials like radamsa crashes)
			rawMaterial, err = os.ReadFile(value)
			if err != nil {
				return nil, fmt.Errorf("failed to read material content: %w", err)
			}
		}
	}

	// special case for ATTESTATION materials, the statement needs to be extracted from the dsse wrapper.
	if m.MaterialType == v1.CraftingSchema_Material_ATTESTATION {
		// support both DSSE envelope and Sigstore bundle
		envelope, err := attestation.ExtractDSSEEnvelope(rawMaterial)
		if err != nil {
			return nil, fmt.Errorf("failed to extract DSSE envelope: %w", err)
		}

		rawMaterial, err = envelope.DecodeB64Payload()
		if err != nil {
			return nil, fmt.Errorf("failed to decode attestation material: %w", err)
		}
	}

	// Non-JSON materials (XML- or text-based formats) are projected to the JSON
	// the policy engine consumes; JSON-native materials pass through unchanged.
	rawMaterial, err = m.ingestMaterialToJSON(rawMaterial, value)
	if err != nil {
		return nil, err
	}

	// if raw material is empty (container images, for example), let's create an empty json
	if len(rawMaterial) == 0 {
		rawMaterial = []byte(`{}`)
	}

	// Decode input as json
	decoder := json.NewDecoder(bytes.NewReader(rawMaterial))
	decoder.UseNumber()

	var decodedMaterial any
	if err = decoder.Decode(&decodedMaterial); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	inputMap := make(map[string]any)
	// if input is an array, set is as an object
	if array, ok := decodedMaterial.([]interface{}); ok {
		inputMap["elements"] = array
	} else if materialAsMap, ok := decodedMaterial.(map[string]any); ok {
		inputMap = materialAsMap
	}

	// Add intoto descriptor
	descriptor, err := m.CraftingStateToIntotoDescriptor("")
	if err != nil {
		return nil, fmt.Errorf("failed to add chainloop descriptor to material: %w", err)
	}
	inputMap["chainloop_metadata"] = descriptor

	// encode back to byte[]
	result, err := json.Marshal(inputMap)
	if err != nil {
		return nil, fmt.Errorf("failed to encode input: %w", err)
	}

	return result, nil
}

// ingestMaterialToJSON projects materials that are not natively JSON (XML- or
// text-based formats) into the JSON structure the policy engine consumes.
// Materials that are already JSON are returned unchanged.
func (m *Attestation_Material) ingestMaterialToJSON(rawMaterial []byte, value string) ([]byte, error) {
	switch m.MaterialType {
	case v1.CraftingSchema_Material_JUNIT_XML:
		suites, err := materialsjunit.Ingest(value)
		if err != nil {
			return nil, fmt.Errorf("failed to ingest junit xml: %w", err)
		}
		// this will render a json array
		return json.Marshal(suites)
	case v1.CraftingSchema_Material_RADAMSA_REPORT:
		// radamsa's -M metadata log is one record per generated iteration; render
		// it as a JSON array so the policy engine exposes it as input.elements.
		//
		// The value may be a single -M log or an archive of per-run logs, so the
		// records are merged across every archive entry here. Doing this only at
		// craft time is not enough: the policy is evaluated against this projection,
		// so an archive that was not expanded here would hand the engine zip/tar.gz
		// bytes, which parse to no records and make the gate skip — a clean-looking
		// false pass. ParseReportBytes shares its detection and parse rules with the
		// crafter's InspectReport so the two cannot disagree.
		records, err := materialsradamsa.ParseReportBytes(rawMaterial)
		if err != nil {
			return nil, fmt.Errorf("invalid radamsa -M metadata log: %w", err)
		}
		return json.Marshal(records)
	case v1.CraftingSchema_Material_TRUFFLEHOG_JSON:
		// TruffleHog --json output is JSONL (one finding per line), not a JSON
		// array; render it as a JSON array so the policy engine exposes the
		// findings as input.elements.
		findings, err := trufflehog.Parse(bytes.NewReader(rawMaterial))
		if err != nil {
			return nil, fmt.Errorf("invalid trufflehog report: %w", err)
		}
		return json.Marshal(findings)
	case v1.CraftingSchema_Material_RADAMSA_CRASHES:
		// metadata-only: the crash content (single binary file or archive) is
		// never evaluated. Discard it so inline content is not parsed as JSON;
		// policies read the crash count from chainloop_metadata.annotations.
		return []byte("{}"), nil
	case v1.CraftingSchema_Material_JACOCO_XML:
		var report jacoco.Report
		if err := xml.Unmarshal(rawMaterial, &report); err != nil {
			return nil, fmt.Errorf("invalid Jacoco report file: %w", err)
		}
		return json.Marshal(&report)
	case v1.CraftingSchema_Material_COBERTURA_XML:
		var report cobertura.Coverage
		if err := xml.Unmarshal(rawMaterial, &report); err != nil {
			return nil, fmt.Errorf("invalid Cobertura report file: %w", err)
		}
		return json.Marshal(&report)
	case v1.CraftingSchema_Material_PITEST_XML:
		var report pitest.Report
		if err := xml.Unmarshal(rawMaterial, &report); err != nil {
			return nil, fmt.Errorf("invalid PIT report file: %w", err)
		}
		return json.Marshal(&report)
	case v1.CraftingSchema_Material_SYSINTERNALS_SIGCHECK:
		report, err := tabular.Parse(rawMaterial)
		if err != nil {
			return nil, fmt.Errorf("failed to ingest sigcheck report: %w", err)
		}
		return report.JSON()
	case v1.CraftingSchema_Material_SYSINTERNALS_ACCESSCHK:
		// AccessChk emits plain text; project it to JSON so the policy engine,
		// which only consumes JSON, can evaluate it. The raw text is preserved
		// in the projection's "raw" field for string-matching fallbacks.
		//
		// The projection de-duplicates security descriptors: a registry hive or
		// service database applies a handful of distinct descriptors to hundreds
		// of thousands of objects, and repeating each one inline would balloon the
		// document the policy engine holds in memory (large materials have
		// OOM-killed CI runners). Objects reference a shared descriptors table by
		// index instead; no object, name, or ACE is dropped, so policy findings
		// are unchanged. Policies read a descriptor via input.descriptors[obj.descriptor].
		report, err := accesschk.Parse(bytes.NewReader(rawMaterial))
		if err != nil {
			return nil, fmt.Errorf("invalid accesschk material: %w", err)
		}
		projection, err := report.Project()
		if err != nil {
			return nil, fmt.Errorf("failed to project accesschk material: %w", err)
		}
		return json.Marshal(projection)
	case v1.CraftingSchema_Material_CERTCC_DRANZER:
		// dranzer emits plain text; project it to JSON so the policy engine,
		// which only consumes JSON, can evaluate it. The raw text is preserved
		// in the projection's "raw" field for string-matching fallbacks.
		//
		// The material may also be an archive of the per-mode reports of one run
		// (-b/-p/-s/-t), so the projection aggregates its entries. Parsing the
		// archive bytes as text instead would yield an empty report, and the
		// policy would then skip rather than evaluate — a false pass.
		bundle, err := dranzer.ParseBundle(rawMaterial)
		if err != nil {
			return nil, fmt.Errorf("invalid dranzer material: %w", err)
		}
		return json.Marshal(bundle)
	}

	return rawMaterial, nil
}

// CraftingStateToIntotoDescriptor creates an intoto descriptor from a material in crafting state
func (m *Attestation_Material) CraftingStateToIntotoDescriptor(name string) (*intoto.ResourceDescriptor, error) {
	material := &intoto.ResourceDescriptor{}

	artifactType := m.MaterialType
	nMaterial, err := m.NormalizedOutput()
	if err != nil {
		return nil, fmt.Errorf("error normalizing material: %w", err)
	}
	if artifactType == v1.CraftingSchema_Material_STRING {
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

	// string materials don't have an artifact nor container, so a name is not available.
	if name == "" {
		name = m.GetId()
	}

	// Required, built-in annotations
	annotationsM := map[string]interface{}{
		AnnotationMaterialType: artifactType.String(),
		AnnotationMaterialName: name,
	}

	// Set the special annotations for container images
	// NOTE: this is in fact an OCI artifact that can be a container image or any stored OCI artifact
	if m.GetContainerImage() != nil {
		if tag := m.GetContainerImage().GetTag(); tag != "" {
			annotationsM[AnnotationContainerTag] = tag
		}

		if sigDigest := m.GetContainerImage().GetSignatureDigest(); sigDigest != "" {
			annotationsM[AnnotationSignatureDigest] = sigDigest
		}

		if sigProvider := m.GetContainerImage().GetSignatureProvider(); sigProvider != "" {
			annotationsM[AnnotationSignatureProvider] = sigProvider
		}

		if sigPayload := m.GetContainerImage().GetSignature(); sigPayload != "" {
			annotationsM[AnnotationMaterialSignature] = sigPayload
		}

		annotationsM[AnnotationsContainerLatestTag] = m.GetContainerImage().GetHasLatestTag().GetValue()
	}

	// Set specials annotations for SBOM artifacts
	if m.GetSbomArtifact() != nil {
		// Main component information
		if mainComponent := m.GetSbomArtifact().GetMainComponent(); mainComponent != nil {
			annotationsM[AnnotationsSBOMMainComponentName] = mainComponent.GetName()
			annotationsM[AnnotationsSBOMMainComponentType] = mainComponent.GetKind()
			annotationsM[AnnotationsSBOMMainComponentVersion] = mainComponent.GetVersion()
		}
	}

	// Custom annotations, it does not override the built-in ones
	for k, v := range m.Annotations {
		_, ok := annotationsM[k]
		if !ok {
			annotationsM[k] = v
		}
	}

	if m.UploadedToCas {
		annotationsM[AnnotationMaterialCAS] = true
	} else if m.InlineCas {
		annotationsM[AnnotationMaterialInlineCAS] = true
	}

	material.Annotations, err = structpb.NewStruct(annotationsM)
	if err != nil {
		return nil, fmt.Errorf("error creating annotations: %w", err)
	}

	return material, nil
}

func CreateAnnotation(name string) string {
	return fmt.Sprintf("%s%s", AnnotationPrefix, name)
}

// GetEnvAllowList returns the environment allow list from either v1 or v2 schema
func (state *CraftingState) GetEnvAllowList() []string {
	switch schema := state.GetSchema().(type) {
	case *CraftingState_InputSchema:
		return schema.InputSchema.GetEnvAllowList()
	case *CraftingState_SchemaV2:
		return schema.SchemaV2.GetSpec().GetEnvAllowList()
	default:
		return nil
	}
}

// GetMaterials returns the materials from either v1 or v2 schema
func (state *CraftingState) GetMaterials() []*v1.CraftingSchema_Material {
	switch schema := state.GetSchema().(type) {
	case *CraftingState_InputSchema:
		return schema.InputSchema.GetMaterials()
	case *CraftingState_SchemaV2:
		return schema.SchemaV2.GetSpec().GetMaterials()
	default:
		return nil
	}
}

// GetAnnotations returns the annotations from either v1 or v2 schema
func (state *CraftingState) GetAnnotations() []*v1.Annotation {
	switch schema := state.GetSchema().(type) {
	case *CraftingState_InputSchema:
		return schema.InputSchema.GetAnnotations()
	case *CraftingState_SchemaV2:
		return schema.SchemaV2.GetSpec().GetAnnotations()
	default:
		return nil
	}
}

// GetPolicyGroups returns the policy groups from either v1 or v2 schema
func (state *CraftingState) GetPolicyGroups() []*v1.PolicyGroupAttachment {
	switch schema := state.GetSchema().(type) {
	case *CraftingState_InputSchema:
		return schema.InputSchema.GetPolicyGroups()
	case *CraftingState_SchemaV2:
		return schema.SchemaV2.GetSpec().GetPolicyGroups()
	default:
		return nil
	}
}

// GetPolicies returns the policies from either v1 or v2 schema
func (state *CraftingState) GetPolicies() *v1.Policies {
	switch schema := state.GetSchema().(type) {
	case *CraftingState_InputSchema:
		return schema.InputSchema.GetPolicies()
	case *CraftingState_SchemaV2:
		return schema.SchemaV2.GetSpec().GetPolicies()
	default:
		return nil
	}
}
