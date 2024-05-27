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

package materials

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/bufbuild/protovalidate-go"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/casclient"
	"github.com/google/go-containerregistry/pkg/authn"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// ErrInvalidMaterialType is returned when the provided material type
	// is not from the kind we are expecting
	ErrInvalidMaterialType = fmt.Errorf("unexpected material type")
	// ErrBaseUploadAndCraft is returned as a base error when the upload and craft of a material fails
	ErrBaseUploadAndCraft = errors.New("upload and craft error")
)

type crafterCommon struct {
	logger *zerolog.Logger
	input  *schemaapi.CraftingSchema_Material
}

// uploadAndCraft uploads the artifact to CAS and crafts the material
// this function is used by all the uploadable artifacts crafters (SBOMs, JUnit, and more in the future)
func uploadAndCraft(ctx context.Context, input *schemaapi.CraftingSchema_Material, backend *casclient.CASBackend, artifactPath string, l *zerolog.Logger) (*api.Attestation_Material, error) {
	// 1 - Check the file can be stored in the provided CAS backend
	result, err := fileStats(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("getting file stats: %w", err)
	}
	defer result.r.Close()

	l.Debug().Str("filename", result.filename).Str("digest", result.digest).Str("path", artifactPath).
		Str("size", bytefmt.ByteSize(uint64(result.size))).
		Str("max_size", bytefmt.ByteSize(uint64(backend.MaxSize))).
		Str("backend", backend.Name).Msg("crafting file")

	// If there is a max size set and the file is bigger than that, return an error
	if backend.MaxSize > 0 && result.size > backend.MaxSize {
		return nil, fmt.Errorf("%w: %w", ErrBaseUploadAndCraft, fmt.Errorf("this file is too big for the %s CAS backend, please contact your administrator: fileSize=%s, maxSize=%s", backend.Name, bytefmt.ByteSize(uint64(result.size)), bytefmt.ByteSize(uint64(backend.MaxSize))))
	}

	material := &api.Attestation_Material{
		MaterialType: input.Type,
		M: &api.Attestation_Material_Artifact_{
			Artifact: &api.Attestation_Material_Artifact{
				Id:        input.Name,
				Name:      result.filename,
				Digest:    result.digest,
				IsSubject: input.Output,
			},
		},
	}

	// 2 - Upload the file to CAS
	if backend.Uploader != nil {
		l.Debug().Str("backend", backend.Name).Msg("uploading")

		_, err = backend.Uploader.UploadFile(ctx, artifactPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBaseUploadAndCraft, fmt.Errorf("uploading material: %w", err))
		}

		material.UploadedToCas = true
	} else {
		l.Debug().Str("backend", backend.Name).Msg("storing inline")
		// or store it inline if no uploader is provided
		content, err := io.ReadAll(result.r)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBaseUploadAndCraft, fmt.Errorf("reading file: %w", err))
		}

		material.InlineCas = true
		material.GetArtifact().Content = content
	}

	return material, nil
}

type fileInfo struct {
	filename, digest string
	size             int64
	r                io.ReadCloser
}

// Returns the sha256 hash of the file, its size and an error
func fileStats(filepath string) (*fileInfo, error) {
	stat, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("can't open file to upload: %w", err)
	}

	hash, _, err := cr_v1.SHA256(f)
	if err != nil {
		return nil, fmt.Errorf("generating digest: %w", err)
	}

	// Since we have already iterated on the file to calculate the digest
	// we need to rewind the file pointer
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("rewinding file pointer: %w", err)
	}

	return &fileInfo{filename: stat.Name(), digest: hash.String(), size: stat.Size(), r: f}, nil
}

type Craftable interface {
	Craft(ctx context.Context, value string) (*api.Attestation_Material, error)
}

func Craft(ctx context.Context, materialSchema *schemaapi.CraftingSchema_Material, value string, casBackend *casclient.CASBackend, ociAuth authn.Keychain, logger *zerolog.Logger) (*api.Attestation_Material, error) {
	var crafter Craftable
	var err error

	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("could not create validator: %w", err)
	}

	if err := validator.Validate(materialSchema); err != nil {
		return nil, fmt.Errorf("validating material: %w", err)
	}

	switch materialSchema.Type {
	case schemaapi.CraftingSchema_Material_STRING:
		crafter, err = NewStringCrafter(materialSchema)
	case schemaapi.CraftingSchema_Material_CONTAINER_IMAGE:
		crafter, err = NewOCIImageCrafter(materialSchema, ociAuth, logger)
	case schemaapi.CraftingSchema_Material_ARTIFACT:
		crafter, err = NewArtifactCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON:
		crafter, err = NewCyclonedxJSONCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON:
		crafter, err = NewSPDXJSONCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_JUNIT_XML:
		crafter, err = NewJUnitXMLCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_OPENVEX:
		crafter, err = NewOpenVEXCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_CSAF_VEX:
		crafter, err = NewCSAFVEXCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_CSAF_INFORMATIONAL_ADVISORY:
		crafter, err = NewCSAFInformationalAdvisoryCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_CSAF_SECURITY_ADVISORY:
		crafter, err = NewCSAFSecurityAdvisoryCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_CSAF_SECURITY_INCIDENT_RESPONSE:
		crafter, err = NewCSAFSecurityIncidentResponseCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_SARIF:
		crafter, err = NewSARIFCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_HELM_CHART:
		crafter, err = NewHelmChartCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_EVIDENCE:
		crafter, err = NewEvidenceCrafter(materialSchema, casBackend, logger)
	case schemaapi.CraftingSchema_Material_ATTESTATION:
		crafter, err = NewAttestationCrafter(materialSchema, casBackend, logger)
	default:
		return nil, fmt.Errorf("material of type %q not supported yet", materialSchema.Type)
	}

	if err != nil {
		return nil, err
	}

	m, err := crafter.Craft(ctx, value)
	if err != nil {
		return nil, fmt.Errorf("crafting material: %w", err)
	}

	m.AddedAt = timestamppb.New(time.Now())
	m.Annotations = make(map[string]string)

	for _, annotation := range materialSchema.Annotations {
		m.Annotations[annotation.Name] = annotation.Value
	}

	return m, nil
}
