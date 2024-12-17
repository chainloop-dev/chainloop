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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/jacoco"
	materialsjunit "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/junit"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/types/known/structpb"
)

const AnnotationPrefix = "chainloop."

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
		return &NormalizedMaterialOutput{Content: []byte(a.Value)}, nil
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

// GetEvaluableContent returns the content to be sent to policy evaluations
func (m *Attestation_Material) GetEvaluableContent(value string) ([]byte, error) {
	var rawMaterial []byte
	var err error

	artifact := m.GetArtifact()
	if artifact == nil && m.GetSbomArtifact() != nil {
		artifact = m.GetSbomArtifact().GetArtifact()
	}

	if artifact != nil {
		if m.InlineCas {
			rawMaterial = artifact.GetContent()
		} else if value == "" {
			return nil, errors.New("artifact path required")
		} else if m.MaterialType != v1.CraftingSchema_Material_HELM_CHART &&
			m.MaterialType != v1.CraftingSchema_Material_JUNIT_XML {
			// read content from local filesystem (except for tgz charts)
			rawMaterial, err = os.ReadFile(value)
			if err != nil {
				return nil, fmt.Errorf("failed to read material content: %w", err)
			}
		}
	}

	// special case for ATTESTATION materials, the statement needs to be extracted from the dsse wrapper.
	if m.MaterialType == v1.CraftingSchema_Material_ATTESTATION {
		var envelope dsse.Envelope
		if err := json.Unmarshal(rawMaterial, &envelope); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attestation material: %w", err)
		}

		rawMaterial, err = envelope.DecodeB64Payload()
		if err != nil {
			return nil, fmt.Errorf("failed to decode attestation material: %w", err)
		}
	}

	// For XML based materials, we need to ingest them and read as json-like structure
	switch m.MaterialType {
	case v1.CraftingSchema_Material_JUNIT_XML:
		suites, err := materialsjunit.Ingest(value)
		if err != nil {
			return nil, fmt.Errorf("failed to ingest junit xml: %w", err)
		}
		// this will render a json array
		rawMaterial, err = json.Marshal(suites)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal junit xml: %w", err)
		}
	case v1.CraftingSchema_Material_JACOCO_XML:
		var report jacoco.Report
		if err := xml.Unmarshal(rawMaterial, &report); err != nil {
			return nil, fmt.Errorf("invalid Jacoco report file: %w", err)
		}
		rawMaterial, err = json.Marshal(&report)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to json Jacoco report file: %w", err)
		}
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
		name = m.GetID()
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

func (m *Attestation_Material) GetID() string {
	if m.GetArtifact() != nil {
		return m.GetArtifact().GetId()
	} else if m.GetContainerImage() != nil {
		return m.GetContainerImage().GetId()
	} else if m.GetSbomArtifact() != nil {
		return m.GetSbomArtifact().GetArtifact().GetId()
	}
	return ""
}

func CreateAnnotation(name string) string {
	return fmt.Sprintf("%s%s", AnnotationPrefix, name)
}
