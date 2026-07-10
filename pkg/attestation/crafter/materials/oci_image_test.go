//
// Copyright 2025 The Chainloop Authors.
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

package materials_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOCIImageCrafter(t *testing.T) {
	testCases := []struct {
		name    string
		input   *contractAPI.CraftingSchema_Material
		wantErr bool
	}{
		{
			name: "container image type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			},
		},
		{
			name: "helm chart type",
			input: &contractAPI.CraftingSchema_Material{
				Type: contractAPI.CraftingSchema_Material_HELM_CHART,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := zerolog.Nop()
			_, err := materials.NewOCIImageCrafter(tc.input, nil, &l)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOCIImageCraft_Layout(t *testing.T) {
	testCases := []struct {
		name       string
		layoutPath string
		wantErr    string
		wantDigest string
		wantName   string
		wantTag    string
	}{
		{
			name:       "crane - single image with annotations",
			layoutPath: "testdata/oci-layouts/crane",
			wantName:   "oci-layout:unknown",
			wantDigest: "sha256:fa6d9058c3d65a33ff565c0e35172f2d99e76fbf8358d91ffaa2208eff2be400",
		},
		{
			name:       "skopeo - single image with tag annotation",
			layoutPath: "testdata/oci-layouts/skopeo",
			wantName:   "oci-layout:v1.51.0",
			wantDigest: "sha256:fa6d9058c3d65a33ff565c0e35172f2d99e76fbf8358d91ffaa2208eff2be400",
		},
		{
			name:       "skopeo-alt - alternative format",
			layoutPath: "testdata/oci-layouts/skopeo-alt",
			wantName:   "oci-layout:v1.51.0",
			wantDigest: "sha256:a5303ef28a4bd9b6e06aa92c07831dd151ac64172695971226bdba4a11fc1b88",
		},
		{
			name:       "non-existent path",
			layoutPath: "/non/existent/path",
			wantErr:    "UNAUTHORIZED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			}
			l := zerolog.Nop()
			crafter, err := materials.NewOCIImageCrafter(schema, nil, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.layoutPath)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_CONTAINER_IMAGE.String(), got.MaterialType.String())

			// Check container image fields
			containerImage := got.GetContainerImage()
			require.NotNil(t, containerImage)
			assert.Equal(t, tc.wantName, containerImage.Name)
			if tc.wantTag != "" {
				assert.Equal(t, tc.wantTag, containerImage.Tag)
			}
			if tc.wantDigest != "" {
				assert.Equal(t, tc.wantDigest, containerImage.Digest)
			} else {
				assert.NotEmpty(t, containerImage.Digest)
			}
		})
	}
}

func TestOCIImageCraft_LayoutWithDigestSelector(t *testing.T) {
	testCases := []struct {
		name           string
		layoutPath     string
		digestSelector string
		wantErr        string
		wantName       string
		wantDigest     string
	}{
		{
			name:           "oras - select first image by digest",
			layoutPath:     "testdata/oci-layouts/oras",
			digestSelector: "sha256:b1747c197a0ab3cb89e109f60a3c5d4ede6946e447fd468fa82d85fa94c6c6e5",
			wantName:       "oci-layout:unknown",
			wantDigest:     "sha256:b1747c197a0ab3cb89e109f60a3c5d4ede6946e447fd468fa82d85fa94c6c6e5",
		},
		{
			name:           "oras - select second image by digest",
			layoutPath:     "testdata/oci-layouts/oras",
			digestSelector: "sha256:f333056ac987169b2a121c16d06112d88ec3d7cb50b098bb17b0f14b0c52f6f3",
			wantName:       "oci-layout:unknown",
			wantDigest:     "sha256:f333056ac987169b2a121c16d06112d88ec3d7cb50b098bb17b0f14b0c52f6f3",
		},
		{
			name:           "zarf - select specific image from bundle",
			layoutPath:     "testdata/oci-layouts/zarf",
			digestSelector: "sha256:e8ac056f7b9b44b07935fe23b8383e5e550d479dc5c6261941e76449a8f7e926",
			wantName:       "oci-layout:ghcr.io/chainloop-dev/chainloop/artifact-cas:v1.51.0",
			wantDigest:     "sha256:e8ac056f7b9b44b07935fe23b8383e5e550d479dc5c6261941e76449a8f7e926",
		},
		{
			name:           "digest not found",
			layoutPath:     "testdata/oci-layouts/oras",
			digestSelector: "sha256:nonexistent",
			wantErr:        "not found in OCI layout",
		},
		{
			name:       "oras - multiple images without digest selector",
			layoutPath: "testdata/oci-layouts/oras",
			wantErr:    "contains 3 images, please specify which one",
		},
		{
			name:       "zarf - multiple images without digest selector",
			layoutPath: "testdata/oci-layouts/zarf",
			wantErr:    "contains 3 images, please specify which one",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			imageRef := tc.layoutPath
			if tc.digestSelector != "" {
				imageRef = tc.layoutPath + "@" + tc.digestSelector
			}

			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			}
			l := zerolog.Nop()
			crafter, err := materials.NewOCIImageCrafter(schema, nil, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), imageRef)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, contractAPI.CraftingSchema_Material_CONTAINER_IMAGE.String(), got.MaterialType.String())

			// Check container image fields
			containerImage := got.GetContainerImage()
			require.NotNil(t, containerImage)
			if tc.wantName != "" {
				assert.Equal(t, tc.wantName, containerImage.Name)
			}
			if tc.wantDigest != "" {
				assert.Equal(t, tc.wantDigest, containerImage.Digest)
			} else {
				assert.NotEmpty(t, containerImage.Digest)
			}
		})
	}
}

func TestOCIImageCraft_RemoteTag(t *testing.T) {
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	repo := fmt.Sprintf("%s/test/image", u.Host)

	for _, tag := range []string{"v1.0.0", "latest"} {
		ref, err := name.ParseReference(fmt.Sprintf("%s:%s", repo, tag))
		require.NoError(t, err)
		require.NoError(t, remote.Write(ref, empty.Image))
	}

	taggedRef, err := name.ParseReference(fmt.Sprintf("%s:v1.0.0", repo))
	require.NoError(t, err)
	desc, err := remote.Get(taggedRef)
	require.NoError(t, err)
	digest := desc.Digest.String()

	testCases := []struct {
		name     string
		imageRef string
		wantTag  string
	}{
		{
			name:     "tagged reference",
			imageRef: fmt.Sprintf("%s:v1.0.0", repo),
			wantTag:  "v1.0.0",
		},
		{
			// a digest reference carries no tag, the digest must not leak into the tag field
			name:     "digest reference",
			imageRef: fmt.Sprintf("%s@%s", repo, digest),
			wantTag:  "",
		},
		{
			name:     "untagged reference defaults to latest",
			imageRef: repo,
			wantTag:  "latest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			}
			l := zerolog.Nop()
			crafter, err := materials.NewOCIImageCrafter(schema, authn.DefaultKeychain, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), tc.imageRef)
			require.NoError(t, err)

			containerImage := got.GetContainerImage()
			require.NotNil(t, containerImage)
			assert.Equal(t, tc.wantTag, containerImage.Tag)
			assert.Equal(t, digest, containerImage.Digest)
		})
	}
}

func TestOCIImageCraft_LayoutTagExtraction(t *testing.T) {
	testCases := []struct {
		name           string
		digestSelector string
		wantTag        string
	}{
		{
			name:           "tagged containerd reference",
			digestSelector: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantTag:        "v2.0.0",
		},
		{
			// a digest reference carries no tag, the digest must not leak into the tag field
			name:           "digest containerd reference",
			digestSelector: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			wantTag:        "",
		},
		{
			// the registry port must not be mistaken for a tag
			name:           "untagged reference on a registry with port",
			digestSelector: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			wantTag:        "latest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			}
			l := zerolog.Nop()
			crafter, err := materials.NewOCIImageCrafter(schema, nil, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), "testdata/oci-layouts/containerd@"+tc.digestSelector)
			require.NoError(t, err)

			containerImage := got.GetContainerImage()
			require.NotNil(t, containerImage)
			assert.Equal(t, tc.wantTag, containerImage.Tag)
		})
	}
}
