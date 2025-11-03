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
	"testing"

	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
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
		name        string
		setupLayout func(t *testing.T) string
		wantErr     string
		wantDigest  string
		wantName    string
		wantTag     string
	}{
		{
			name: "valid OCI layout",
			setupLayout: func(t *testing.T) string {
				return createTestOCILayout(t, "test-image", "v1.0.0")
			},
			wantName: "test-image",
			wantTag:  "v1.0.0",
		},
		{
			name: "OCI layout without annotations",
			setupLayout: func(t *testing.T) string {
				return createTestOCILayout(t, "", "")
			},
			wantName: "oci-layout",
			wantTag:  "",
		},
		{
			name: "non-existent path",
			setupLayout: func(_ *testing.T) string {
				return "/non/existent/path"
			},
			wantErr: "UNAUTHORIZED",
		},
		{
			name: "empty directory",
			setupLayout: func(t *testing.T) string {
				dir := t.TempDir()
				return dir
			},
			wantErr: "could not parse reference",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			layoutPath := tc.setupLayout(t)

			schema := &contractAPI.CraftingSchema_Material{
				Name: "test",
				Type: contractAPI.CraftingSchema_Material_CONTAINER_IMAGE,
			}
			l := zerolog.Nop()
			crafter, err := materials.NewOCIImageCrafter(schema, nil, &l)
			require.NoError(t, err)

			got, err := crafter.Craft(context.TODO(), layoutPath)
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
			assert.Equal(t, tc.wantTag, containerImage.Tag)
			assert.NotEmpty(t, containerImage.Digest)
			assert.True(t, len(containerImage.Digest) > 0, "digest should not be empty")
		})
	}
}

func TestOCIImageCraft_LayoutWithDigestSelector(t *testing.T) {
	testCases := []struct {
		name        string
		setupLayout func(t *testing.T) (string, string) // returns (layoutPath, digestToSelect)
		wantErr     string
		wantName    string
		wantTag     string
	}{
		{
			name: "select second image by digest",
			setupLayout: func(t *testing.T) (string, string) {
				layoutPath, digests := createTestOCILayoutMultiple(t, []imageSpec{
					{name: "first-image", tag: "v1.0.0"},
					{name: "second-image", tag: "v2.0.0"},
				})
				return layoutPath, digests[1] // Select second image
			},
			wantName: "second-image",
			wantTag:  "v2.0.0",
		},
		{
			name: "digest not found",
			setupLayout: func(t *testing.T) (string, string) {
				layoutPath, _ := createTestOCILayoutMultiple(t, []imageSpec{
					{name: "test-image", tag: "v1.0.0"},
				})
				return layoutPath, "sha256:nonexistent"
			},
			wantErr: "not found in OCI layout",
		},
		{
			name: "multiple images without digest selector",
			setupLayout: func(t *testing.T) (string, string) {
				layoutPath, _ := createTestOCILayoutMultiple(t, []imageSpec{
					{name: "first-image", tag: "v1.0.0"},
					{name: "second-image", tag: "v2.0.0"},
					{name: "third-image", tag: "v3.0.0"},
				})
				return layoutPath, "" // No digest selector
			},
			wantErr: "contains 3 images, please specify which one",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			layoutPath, digest := tc.setupLayout(t)
			imageRef := layoutPath
			if digest != "" {
				imageRef = layoutPath + "@" + digest
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
			assert.Equal(t, tc.wantName, containerImage.Name)
			assert.Equal(t, tc.wantTag, containerImage.Tag)
			assert.NotEmpty(t, containerImage.Digest)
		})
	}
}

type imageSpec struct {
	name string
	tag  string
}

// createTestOCILayoutMultiple creates an OCI layout with multiple images for testing
func createTestOCILayoutMultiple(t *testing.T, specs []imageSpec) (string, []string) {
	t.Helper()

	layoutPath := t.TempDir()
	path, err := layout.Write(layoutPath, empty.Index)
	require.NoError(t, err)

	digests := make([]string, 0, len(specs))
	for _, spec := range specs {
		img, err := random.Image(1024, 1)
		require.NoError(t, err)

		var opts []layout.Option
		if spec.name != "" || spec.tag != "" {
			annotations := make(map[string]string)
			if spec.name != "" {
				annotations["org.opencontainers.image.ref.name"] = spec.name
			}
			if spec.tag != "" {
				annotations["io.containerd.image.name"] = spec.name + ":" + spec.tag
			}
			opts = append(opts, layout.WithAnnotations(annotations))
		}

		err = path.AppendImage(img, opts...)
		require.NoError(t, err)

		// Get the digest of the image we just added
		index, err := path.ImageIndex()
		require.NoError(t, err)
		manifest, err := index.IndexManifest()
		require.NoError(t, err)
		// The last manifest is the one we just added
		digests = append(digests, manifest.Manifests[len(manifest.Manifests)-1].Digest.String())
	}

	return layoutPath, digests
}

// createTestOCILayout creates a minimal valid OCI layout directory for testing
func createTestOCILayout(t *testing.T, imageName, tag string) string {
	t.Helper()

	layoutPath := t.TempDir()

	// Use go-containerregistry to create a random image
	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	// Write layout with empty index first
	path, err := layout.Write(layoutPath, empty.Index)
	require.NoError(t, err)

	// Append the image with annotations if provided
	var opts []layout.Option
	if imageName != "" || tag != "" {
		annotations := make(map[string]string)
		if imageName != "" {
			annotations["org.opencontainers.image.ref.name"] = imageName
		}
		if tag != "" {
			annotations["io.containerd.image.name"] = imageName + ":" + tag
		}
		opts = append(opts, layout.WithAnnotations(annotations))
	}

	err = path.AppendImage(img, opts...)
	require.NoError(t, err)

	return layoutPath
}
