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
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/wrapperspb"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"
	cosigntypes "github.com/sigstore/cosign/v2/pkg/types"
)

const (
	cosignSignatureProvider signatureProvider = "cosign"
	notarySignatureProvider signatureProvider = "notary"

	// notarySignatureMimeType is the MIME type for Notary signatures on OCI artifacts.
	// https://github.com/notaryproject/specifications/blob/main/specs/signature-specification.md#oci-signatures
	notarySignatureMimeType = "application/vnd.cncf.notary.signature"
	// latestTag is the tag name for the latest image.
	latestTag = "latest"
)

// signatureProvider is the type for the signature provider of a container image.
type signatureProvider string

// containerSignatureInfo holds the digest and signature provider for a container image.
type containerSignatureInfo struct {
	// digest is the digest of the OCI artifact containing the signature.
	digest string
	// provider is the signature provider.
	provider signatureProvider
	// payload is the base64-encoded OCI artifact manifest containing the signature evidence.
	payload string
}

type OCIImageCrafter struct {
	*crafterCommon
	keychain authn.Keychain
	// validate the artifact type (optional)
	artifactTypeValidation string
}
type OCICraftOpt func(*OCIImageCrafter)

func WithArtifactTypeValidation(artifactTypeValidation string) OCICraftOpt {
	return func(opts *OCIImageCrafter) {
		opts.artifactTypeValidation = artifactTypeValidation
	}
}

func NewOCIImageCrafter(schema *schemaapi.CraftingSchema_Material, ociAuth authn.Keychain, l *zerolog.Logger, opts ...OCICraftOpt) (*OCIImageCrafter, error) {
	craftCommon := &crafterCommon{logger: l, input: schema}
	c := &OCIImageCrafter{crafterCommon: craftCommon, keychain: ociAuth}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
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

	if i.artifactTypeValidation != "" {
		i.logger.Debug().Str("name", imageRef).Str("want", i.artifactTypeValidation).Msg("validating artifact type")
		if descriptor.ArtifactType != i.artifactTypeValidation {
			return nil, fmt.Errorf("artifact type %s does not match expected type %s", descriptor.ArtifactType, i.artifactTypeValidation)
		}
	}

	remoteRef := ref.Context().Digest(descriptor.Digest.String())

	// FQDN of the repo, i.e bitnami/nginx => index.docker.io/bitnami/nginx
	repoName := remoteRef.Repository.String()
	i.logger.Debug().Str("name", repoName).Str("digest", remoteRef.DigestStr()).Msg("container image resolved")

	// Check if the signature tag exists for the image digest
	signatureInfo := i.checkForSignature(ref, descriptor)

	// Check if the image is tagged as "latest"
	hasLatestTag := i.isLatestTag(ref, remoteRef.DigestStr())

	containerImage := &api.Attestation_Material_ContainerImage{
		// TODO: remove once we know servers are not running server-side validation
		Id:           i.input.Name,
		Name:         repoName,
		Digest:       remoteRef.DigestStr(),
		IsSubject:    i.input.Output,
		Tag:          ref.Identifier(),
		HasLatestTag: wrapperspb.Bool(hasLatestTag),
	}

	// If the signature digest exists, add it to the material
	if signatureInfo != nil {
		containerImage.SignatureDigest = signatureInfo.digest
		containerImage.Signature = signatureInfo.payload
		containerImage.SignatureProvider = string(signatureInfo.provider)
	}

	return &api.Attestation_Material{
		MaterialType: i.input.Type,
		M: &api.Attestation_Material_ContainerImage_{
			ContainerImage: containerImage},
	}, nil
}

// checkForSignature checks for a signature for the given image reference.
// Right now it checks for Cosign signatures only by tag or referrers API.
func (i *OCIImageCrafter) checkForSignature(originalRef name.Reference, originalDesc *remote.Descriptor) *containerSignatureInfo {
	var wg sync.WaitGroup
	resultChan := make(chan *containerSignatureInfo, 1)

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

// checkForSignatureTag checks for a signature tag associated with the image digest.
func (i *OCIImageCrafter) checkForSignatureTag(originalRef name.Reference, originalDesc *remote.Descriptor) *containerSignatureInfo {
	trimmedDigest := strings.TrimPrefix(originalDesc.Digest.String(), "sha256:")
	newTag := fmt.Sprintf("sha256-%s.sig", trimmedDigest)

	newRef, err := name.NewTag(fmt.Sprintf("%s:%s", originalRef.Context().Name(), newTag))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to create new tag reference")
		return nil
	}

	desc, err := remote.Get(newRef, remote.WithAuthFromKeychain(i.keychain))
	if err == nil {
		i.logger.Debug().Str("name", newRef.String()).Msg("image signature found")
		return &containerSignatureInfo{
			digest:   desc.Digest.String(),
			provider: cosignSignatureProvider,
			payload:  base64.StdEncoding.EncodeToString(desc.Manifest),
		}
	}
	return nil
}

// checkForSignatureReferrers checks for a signature using the referrers API.
func (i *OCIImageCrafter) checkForSignatureReferrers(originalRef name.Reference, originalDesc *remote.Descriptor) *containerSignatureInfo {
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

	return i.findSignatureInManifest(manifest.Manifests, originalRef)
}

// findSignatureInManifest scans the manifests to identify and retrieve signature information.
func (i *OCIImageCrafter) findSignatureInManifest(manifests []v1.Descriptor, originalRef name.Reference) *containerSignatureInfo {
	for _, m := range manifests {
		if m.ArtifactType == cosigntypes.SimpleSigningMediaType {
			i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Cosign signature artifact using referrers API")
			return i.fetchSignatureManifest(originalRef, m.Digest.String(), cosignSignatureProvider)
		}
		if m.ArtifactType == notarySignatureMimeType {
			i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Notary signature artifact using referrers API")
			return i.fetchSignatureManifest(originalRef, m.Digest.String(), notarySignatureProvider)
		}
	}
	return nil
}

// fetchSignatureManifest retrieves and base64-encodes the signature manifest.
func (i *OCIImageCrafter) fetchSignatureManifest(originalRef name.Reference, digest string, provider signatureProvider) *containerSignatureInfo {
	signatureManifest, err := remote.Get(originalRef.Context().Digest(digest), remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to fetch signature manifest")
		return nil
	}
	return &containerSignatureInfo{
		digest:   digest,
		provider: provider,
		payload:  base64.StdEncoding.EncodeToString(signatureManifest.Manifest),
	}
}

// isLatestTag checks if the image has been tagged with the "latest" tag.
func (i *OCIImageCrafter) isLatestTag(ref name.Reference, currentDigest string) bool {
	// Check first if the image has a "latest" tag already
	if ref.Identifier() == latestTag {
		i.logger.Debug().Str("name", ref.String()).Msg("image has a 'latest' tag")
		return true
	}
	// Try to retrieve the same image with the "latest" tag
	latestRef, err := name.NewTag(fmt.Sprintf("%s:%s", ref.Context().Name(), latestTag))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to create new tag reference")
		return false
	}
	// Check if the "latest" tag exists and points to the same digest
	latestDesc, err := remote.Get(latestRef, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to fetch latest tag or the tag does not exist")
		return false
	}
	// If the "latest" tag points to the same digest, it's a "latest" tag
	if latestDesc.Digest.String() == currentDigest {
		i.logger.Debug().Str("name", latestRef.String()).Msg("image has a 'latest' tag")
		return true
	}

	i.logger.Debug().Str("name", latestRef.String()).Msg("image does not have a 'latest' tag")
	return false
}
