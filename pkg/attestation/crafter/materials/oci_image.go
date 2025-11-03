//
// Copyright 2024-2025 The Chainloop Authors.
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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/wrapperspb"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	api "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
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
	// ociLayoutRepoName is the default repository name for OCI layout images.
	ociLayoutRepoName = "oci-layout"
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

func (i *OCIImageCrafter) Craft(ctx context.Context, imageRef string) (*api.Attestation_Material, error) {
	// Check if imageRef is a path to an OCI layout directory
	layoutPath, digestSelector := parseLayoutReference(imageRef)
	if i.isOCILayoutPath(layoutPath) {
		i.logger.Debug().Str("path", layoutPath).Str("digest", digestSelector).Msg("detected OCI layout directory")
		return i.craftFromLayout(ctx, layoutPath, digestSelector)
	}

	// Otherwise, treat as remote registry reference
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

// parseLayoutReference parses a layout reference that may include a digest selector.
func parseLayoutReference(ref string) (string, string) {
	// Check for @digest suffix
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}

// isOCILayoutPath checks if the given path is a valid OCI layout directory.
func (i *OCIImageCrafter) isOCILayoutPath(path string) bool {
	// Check if path exists and is a directory
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for oci-layout file
	layoutFile := filepath.Join(path, ociLayoutRepoName)
	if _, err := os.Stat(layoutFile); err != nil {
		return false
	}

	return true
}

// craftFromLayout creates a material from an OCI layout directory.
// If digestSelector is provided, it will look for that specific digest in the layout.
// Otherwise, it uses the first manifest in the index.
func (i *OCIImageCrafter) craftFromLayout(_ context.Context, layoutPath, digestSelector string) (*api.Attestation_Material, error) {
	// Read the OCI layout
	layoutPath, err := filepath.Abs(layoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCI layout: %w", err)
	}

	// Get the image index
	index, err := path.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to read image index: %w", err)
	}

	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read index manifest: %w", err)
	}

	if len(indexManifest.Manifests) == 0 {
		return nil, fmt.Errorf("no manifests found in OCI layout")
	}

	// Select the manifest based on digest selector
	// If a specific digest is requested, find it
	if digestSelector != "" {
		found := false
		var manifest v1.Descriptor
		for _, m := range indexManifest.Manifests {
			if m.Digest.String() == digestSelector {
				manifest = m
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("digest %s not found in OCI layout", digestSelector)
		}
		i.logger.Debug().Str("digest", digestSelector).Msg("selected image by digest")

		return i.buildMaterialFromManifest(layoutPath, manifest, indexManifest.Manifests)
	}

	// No digest specified - if multiple images exist, require explicit selection
	if len(indexManifest.Manifests) > 1 {
		var digests []string
		for _, m := range indexManifest.Manifests {
			digests = append(digests, m.Digest.String())
		}
		return nil, fmt.Errorf("OCI layout contains %d images, please specify which one to use with @digest. Available digests: %s",
			len(indexManifest.Manifests), strings.Join(digests, ", "))
	}

	// Only one image, safe to use it
	manifest := indexManifest.Manifests[0]
	i.logger.Debug().Msg("using only image in layout")

	return i.buildMaterialFromManifest(layoutPath, manifest, indexManifest.Manifests)
}

// buildMaterialFromManifest constructs the attestation material from a manifest descriptor.
func (i *OCIImageCrafter) buildMaterialFromManifest(layoutPath string, manifest v1.Descriptor, allManifests []v1.Descriptor) (*api.Attestation_Material, error) {

	digest := manifest.Digest.String()

	// Extract repository name from annotations if available
	repoName := ociLayoutRepoName + ":"
	if manifest.Annotations != nil {
		// Try annotation keys in preference order
		for _, key := range []string{
			"org.opencontainers.image.ref.name",
			"org.opencontainers.image.base.name",
		} {
			if name, ok := manifest.Annotations[key]; ok {
				repoName = repoName + name
				break
			}
		}
	}

	// Extract tag from annotations
	tag := ""
	if manifest.Annotations != nil {
		if t, ok := manifest.Annotations["io.containerd.image.name"]; ok {
			// Extract tag from full reference (e.g., "registry/repo:tag" -> "tag")
			parts := strings.Split(t, ":")
			if len(parts) > 1 {
				tag = parts[len(parts)-1]
			}
		}
	}

	// Validate artifact type if specified
	if i.artifactTypeValidation != "" {
		i.logger.Debug().Str("path", layoutPath).Str("want", i.artifactTypeValidation).Msg("validating artifact type")
		if manifest.ArtifactType != i.artifactTypeValidation {
			return nil, fmt.Errorf("artifact type %s does not match expected type %s", manifest.ArtifactType, i.artifactTypeValidation)
		}
	}

	i.logger.Debug().Str("path", layoutPath).Str("digest", digest).Msg("OCI layout image resolved")

	// Check for signatures in the layout
	signatureInfo := i.checkForSignatureInLayout(allManifests, digest)

	containerImage := &api.Attestation_Material_ContainerImage{
		Id:        i.input.Name,
		Name:      repoName,
		Digest:    digest,
		IsSubject: i.input.Output,
		Tag:       tag,
	}

	// Add signature information if found
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

// checkForSignatureInLayout checks for signatures in the OCI layout manifests.
func (i *OCIImageCrafter) checkForSignatureInLayout(manifests []v1.Descriptor, imageDigest string) *containerSignatureInfo {
	// Look for signature artifacts that reference the image digest
	for _, m := range manifests {
		// Check if this manifest references our image
		if m.Annotations != nil {
			if subject, ok := m.Annotations["org.opencontainers.image.base.digest"]; ok && subject == imageDigest {
				// Check for Cosign signature
				if m.ArtifactType == cosigntypes.SimpleSigningMediaType {
					i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Cosign signature artifact in OCI layout")
					return i.encodeLayoutSignature(m, cosignSignatureProvider)
				}
				// Check for Notary signature
				if m.ArtifactType == notarySignatureMimeType {
					i.logger.Debug().Str("digest", m.Digest.String()).Msg("found Notary signature artifact in OCI layout")
					return i.encodeLayoutSignature(m, notarySignatureProvider)
				}
			}
		}
	}

	i.logger.Debug().Str("digest", imageDigest).Msg("no signature found in OCI layout")
	return nil
}

// encodeLayoutSignature encodes a signature descriptor as base64.
func (i *OCIImageCrafter) encodeLayoutSignature(desc v1.Descriptor, provider signatureProvider) *containerSignatureInfo {
	// Marshal the descriptor to JSON for the payload
	manifestBytes, err := json.Marshal(desc)
	if err != nil {
		i.logger.Debug().Err(err).Msg("failed to marshal signature descriptor")
		return nil
	}

	return &containerSignatureInfo{
		digest:   desc.Digest.String(),
		provider: provider,
		payload:  base64.StdEncoding.EncodeToString(manifestBytes),
	}
}
