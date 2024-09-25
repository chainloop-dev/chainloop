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
	"fmt"
	"strings"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"
	cosigntypes "github.com/sigstore/cosign/v2/pkg/types"
)

type OCIImageCrafter struct {
	*crafterCommon
	keychain authn.Keychain
}

func NewOCIImageCrafter(schema *schemaapi.CraftingSchema_Material, ociAuth authn.Keychain, l *zerolog.Logger) (*OCIImageCrafter, error) {
	if schema.Type != schemaapi.CraftingSchema_Material_CONTAINER_IMAGE {
		return nil, fmt.Errorf("material type is not container image")
	}

	craftCommon := &crafterCommon{logger: l, input: schema}
	return &OCIImageCrafter{craftCommon, ociAuth}, nil
}

func (i *OCIImageCrafter) Craft(_ context.Context, imageRef string) (*api.Attestation_Material, error) {
	i.logger.Debug().Str("name", imageRef).Msg("retrieving container image digest from remote")

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, err
	}

	descriptor, err := remote.Get(ref, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		return nil, err
	}

	remoteRef := ref.Context().Digest(descriptor.Digest.String())

	// FQDN of the repo, i.e bitnami/nginx => index.docker.io/bitnami/nginx
	repoName := remoteRef.Repository.String()
	i.logger.Debug().Str("name", repoName).Str("digest", remoteRef.DigestStr()).Msg("container image resolved")

	// Check if the signature tag exists for the image digest
	signDigest := i.CheckForSignature(ref, descriptor)

	containerImage := &api.Attestation_Material_ContainerImage{
		Id: i.input.Name, Name: repoName, Digest: remoteRef.DigestStr(), IsSubject: i.input.Output,
		Tag: ref.Identifier(),
	}

	// If the signature digest exists, add it to the material
	if signDigest != "" {
		containerImage.SignatureDigest = signDigest
	}

	return &api.Attestation_Material{
		MaterialType: i.input.Type,
		M:            &api.Attestation_Material_ContainerImage_{ContainerImage: containerImage},
	}, nil
}

// CheckForSignature checks if the signature tag exists for the image digest.
// It first checks for a tag with the signature digest and, if not found, uses the referrers API to check for related material.
func (i *OCIImageCrafter) CheckForSignature(originalRef name.Reference, originalDesc *remote.Descriptor) string {
	trimmedDigest := strings.TrimPrefix(originalDesc.Digest.String(), "sha256:")
	newTag := fmt.Sprintf("sha256-%s.sig", trimmedDigest)

	// Create new reference for the signature tag
	newRef, err := name.NewTag(fmt.Sprintf("%s:%s", originalRef.Context().Name(), newTag))
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to create new tag reference")
		return ""
	}

	// Try fetching the signature tag
	if desc, err := remote.Head(newRef); err == nil {
		i.logger.Debug().Str("name", newRef.String()).Msg("image signature found")
		return strings.TrimPrefix(desc.Digest.String(), "sha256:")
	}

	i.logger.Debug().Err(err).Str("name", newRef.String()).Msg("image signature not found")

	// Use the referrers API to look for related materials if the tag is not found
	return i.checkReferrersForSignature(originalRef, originalDesc)
}

// checkReferrersForSignature uses the referrers API to find a matching signature artifact.
func (i *OCIImageCrafter) checkReferrersForSignature(originalRef name.Reference, originalDesc *remote.Descriptor) string {
	ref := originalRef.Context().Digest(originalDesc.Digest.String())
	index, err := remote.Referrers(ref, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to fetch referrers")
		return ""
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		i.logger.Error().Err(err).Msg("failed to fetch manifest")
		return ""
	}

	for _, m := range manifest.Manifests {
		// Check if the artifact is a Cosign signature
		if m.ArtifactType == cosigntypes.SimpleSigningMediaType {
			i.logger.Debug().Str("digest", m.Digest.String()).Msg("found signature artifact using referrers API")
			return strings.TrimPrefix(m.Digest.String(), "sha256:")
		}
	}

	return ""
}
