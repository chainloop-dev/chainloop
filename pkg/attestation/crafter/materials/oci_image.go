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
	"sync"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"
	cosigntypes "github.com/sigstore/cosign/v2/pkg/types"
)

const (
	cosignSignatureProvider SignatureProvider = "cosign"
	notarySignatureProvider SignatureProvider = "notary"

	// notarySignatureMimeType is the MIME type for Notary signatures on OCI artifacts.
	// https://github.com/notaryproject/specifications/blob/main/specs/signature-specification.md#oci-signatures
	notarySignatureMimeType = "application/vnd.cncf.notary.signature"
)

// SignatureProvider is the type for the signature provider of a container image.
type SignatureProvider string

// ContainerSignatureInfo holds the digest and signature provider for a container image.
type ContainerSignatureInfo struct {
	digest   string
	provider SignatureProvider
}

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
	signatureInfo := i.checkForSignature(ref, descriptor)

	containerImage := &api.Attestation_Material_ContainerImage{
		Id: i.input.Name, Name: repoName, Digest: remoteRef.DigestStr(), IsSubject: i.input.Output,
		Tag: ref.Identifier(),
	}

	// If the signature digest exists, add it to the material
	if signatureInfo != nil {
		containerImage.SignatureDigest = signatureInfo.digest
		containerImage.SignatureProvider = string(signatureInfo.provider)
	}

	return &api.Attestation_Material{
		MaterialType: i.input.Type,
		M:            &api.Attestation_Material_ContainerImage_{ContainerImage: containerImage},
	}, nil
}

// checkForSignature checks for a signature for the given image reference.
// Right now it checks for Cosign signatures only by tag or referrers API.
func (i *OCIImageCrafter) checkForSignature(originalRef name.Reference, originalDesc *remote.Descriptor) *ContainerSignatureInfo {
	var wg sync.WaitGroup
	resultChan := make(chan *ContainerSignatureInfo, 1)

	// Launch parallel goroutines for signature checks
	wg.Add(2)
	go func() {
		defer wg.Done()
		if sigInfo := i.checkForSignatureTag(originalRef, originalDesc); sigInfo != nil {
			select {
			case resultChan <- sigInfo:
			default:
				// Channel already closed, result obtained from another source
			}
		}
	}()

	go func() {
		defer wg.Done()
		if sigInfo := i.checkForSignatureReferrers(originalRef, originalDesc); sigInfo != nil {
			select {
			case resultChan <- sigInfo:
			default:
				// Channel already closed, result obtained from another source
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first valid signature result
	for sigInfo := range resultChan {
		return sigInfo
	}

	i.logger.Debug().Str("name", originalRef.String()).Msg("image signature not found")
	return nil
}

// checkForSignatureTag checks for a signature tag.
func (i *OCIImageCrafter) checkForSignatureTag(originalRef name.Reference, originalDesc *remote.Descriptor) *ContainerSignatureInfo {
	trimmedDigest := strings.TrimPrefix(originalDesc.Digest.String(), "sha256:")
	newTag := fmt.Sprintf("sha256-%s.sig", trimmedDigest)

	// Create new reference for the signature tag
	newRef, err := name.NewTag(fmt.Sprintf("%s:%s", originalRef.Context().Name(), newTag))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to create new tag reference")
		return nil
	}

	// Try fetching the signature tag
	if desc, err := remote.Head(newRef, remote.WithAuthFromKeychain(i.keychain)); err == nil {
		i.logger.Debug().Str("name", newRef.String()).Msg("image signature found")
		return &ContainerSignatureInfo{digest: desc.Digest.String(), provider: cosignSignatureProvider}
	}

	return nil
}

// checkForSignatureReferrers checks for a signature using the referrers API.
func (i *OCIImageCrafter) checkForSignatureReferrers(originalRef name.Reference, originalDesc *remote.Descriptor) *ContainerSignatureInfo {
	ref := originalRef.Context().Digest(originalDesc.Digest.String())
	index, err := remote.Referrers(ref, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to fetch referrers")
		return nil
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to fetch manifest")
		return nil
	}

	for _, m := range manifest.Manifests {
		// Check if the artifact is a Cosign signature
		if m.ArtifactType == cosigntypes.SimpleSigningMediaType {
			i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Cosign signature artifact using referrers API")
			return &ContainerSignatureInfo{digest: m.Digest.String(), provider: cosignSignatureProvider}
		}

		// Check if the artifact is a Notary signature
		if m.ArtifactType == notarySignatureMimeType {
			i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Notary signature artifact using referrers API")
			return &ContainerSignatureInfo{digest: m.Digest.String(), provider: notarySignatureProvider}
		}
	}

	return nil
}
