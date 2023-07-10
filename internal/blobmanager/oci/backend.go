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

package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
)

type Backend struct {
	keychain Keychain
	prefix   string
	repo     string
}

type Keychain = authn.Keychain

type RegistryOptions struct {
	AllowInsecure bool
	Keychain      Keychain
}

type NewBackendOpt func(*Backend)

func WithPrefix(prefix string) NewBackendOpt {
	return func(b *Backend) {
		b.prefix = prefix
	}
}

const defaultPrefix = "chainloop"

func NewBackend(repository string, regOpts *RegistryOptions, opts ...NewBackendOpt) (*Backend, error) {
	b := &Backend{
		repo:     repository,
		prefix:   defaultPrefix,
		keychain: regOpts.Keychain,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

// Exists check that the artifact is already present in the repository and it points to the
// same image digest, meaning it has not been re-pushed/replaced
// This method is very naive so signatures will be used in future releases
func (b *Backend) Exists(_ context.Context, digest string) (bool, error) {
	if digest == "" {
		return false, errors.New("digest is empty")
	}

	ref, err := name.ParseReference(b.resourcePath(digest))
	if err != nil {
		return false, err
	}

	// It's not trivial to catch if the error is a 404 (yeah I know...) so we will assume that
	// any error means no and will be caught in the next stage when we try to upload the image
	image, err := remote.Image(ref, remote.WithAuthFromKeychain(b.keychain))
	if err != nil {
		// Image is not there
		return false, nil
	}

	// If the image is not a valid chainloop image we will return false
	if err := validateImage(image, digest); err != nil {
		return false, nil
	}

	return true, nil
}

func (b *Backend) Upload(_ context.Context, r io.Reader, resource *pb.CASResource) error {
	// We need to read the whole content before uploading it to the registry
	// This is due to the fact that our OCI push implementation does not support streaming/chunks for uncompressed layers
	// We can not use stream.Layer since it only supports compressed layers, we want to store raw data and set custom mimetypes
	// https://github.com/google/go-containerregistry/blob/main/pkg/v1/stream/README.md
	// TODO: Split content in multiple layers and do concurrent uploads/downloads
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading content: %w", err)
	}

	ref, err := name.ParseReference(b.resourcePath(resource.Digest))
	if err != nil {
		return fmt.Errorf("parsing reference: %w", err)
	}

	img, err := craftImage(data, resource)
	if err != nil {
		return fmt.Errorf("crafting image: %w", err)
	}

	if err := validateImage(img, resource.Digest); err != nil {
		return fmt.Errorf("validating image: %w", err)
	}

	err = remote.Write(ref, img, remote.WithAuthFromKeychain(b.keychain))
	if err != nil {
		return fmt.Errorf("writing image: %w", err)
	}

	return nil
}

func (b *Backend) resourcePath(resourceName string) string {
	return fmt.Sprintf("%s/%s-%s", b.repo, b.prefix, resourceName)
}

const authorAnnotation = "chainloop.dev"

func craftImage(content []byte, resource *pb.CASResource) (v1.Image, error) {
	if len(content) == 0 {
		return nil, errors.New("content is empty")
	}

	if resource == nil || resource.FileName == "" || resource.Digest == "" {
		return nil, errors.New("resource metadata is not valid")
	}

	base := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	base = mutate.ConfigMediaType(base, types.OCIConfigJSON)
	base = mutate.Annotations(base, map[string]string{
		ocispec.AnnotationAuthors: authorAnnotation,
		// TODO: Move this annotation to layer
		ocispec.AnnotationTitle: resource.FileName,
	}).(v1.Image)

	layer := static.NewLayer(content, detectedMediaType(content))
	img, err := mutate.Append(base, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Detect the media type based on the provided content
func detectedMediaType(b []byte) types.MediaType {
	return types.MediaType(strings.Split(http.DetectContentType(b), ";")[0])
}

func (b *Backend) Describe(_ context.Context, digest string) (*pb.CASResource, error) {
	if digest == "" {
		return nil, errors.New("digest is empty")
	}

	ref, err := name.ParseReference(b.resourcePath(digest))
	if err != nil {
		return nil, fmt.Errorf("parsing reference: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(b.keychain))
	if err != nil {
		var e *transport.Error
		if errors.As(err, &e) && e.StatusCode == http.StatusNotFound {
			return nil, backend.NewErrNotFound("image")
		}

		return nil, fmt.Errorf("getting image: %w", err)
	}

	if err := validateImage(img, digest); err != nil {
		return nil, fmt.Errorf("validating image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("extracting manifest: %w", err)
	}

	// Validate image already checked that the manifest has exactly one layer
	size := manifest.Layers[0].Size

	filename, ok := manifest.Annotations[ocispec.AnnotationTitle]
	if !ok {
		return nil, errors.New("couldn't find file metadata")
	}

	return &pb.CASResource{Digest: digest, FileName: filename, Size: size}, nil
}

func (b *Backend) Download(_ context.Context, w io.Writer, digest string) error {
	if digest == "" {
		return errors.New("digest is empty")
	}

	ref, err := name.ParseReference(b.resourcePath(digest))
	if err != nil {
		return fmt.Errorf("parsing reference: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(b.keychain))
	if err != nil {
		return fmt.Errorf("getting image: %w", err)
	}

	if err := validateImage(img, digest); err != nil {
		return fmt.Errorf("validating image: %w", err)
	}

	// Download the layer with the same digest, not relying on the image name
	l, err := img.LayerByDiffID(v1.Hash{Algorithm: "sha256", Hex: digest})
	if err != nil {
		return fmt.Errorf("getting layer with hash sha256:%s: %w", digest, err)
	}

	// Do not uncompress since we want the raw stored data
	rc, err := l.Compressed()
	if err != nil {
		return fmt.Errorf("extracting data from layer: %w", err)
	}

	defer rc.Close()
	// 1MB buffer
	buf := make([]byte, 1<<20)
	_, err = io.CopyBuffer(w, rc, buf)
	if err != nil {
		return fmt.Errorf("copying data from layer: %w", err)
	}

	return nil
}

// validateImage checks that the image was crafted by chainloop and contains the expected content
func validateImage(img v1.Image, digest string) error {
	// Review required annotations
	m, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("getting manifest: %w", err)
	}

	if v, ok := m.Annotations[ocispec.AnnotationAuthors]; !ok || v != authorAnnotation {
		return errors.New("image not uploaded by chainloop")
	}

	if v, ok := m.Annotations[ocispec.AnnotationTitle]; !ok && v != "" {
		return errors.New("image does not contain filename information")
	}

	// NOTE: we use img.Layers instead of LayerByDiffID because the latter does not compute the image manifest
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}

	if len(layers) != 1 {
		return errors.New("image does not contain a single layer")
	}

	// Check the actual layer digest content meets the expected one
	d, err := layers[0].Digest()
	if err != nil {
		return fmt.Errorf("getting layer digest: %w", err)
	}

	if d.Hex != digest {
		return errors.New("layer digest does not match the expected one")
	}

	return nil
}

// CheckWritePermissions performs an actual write to the repository to check that the credentials
func (b *Backend) CheckWritePermissions(_ context.Context) error {
	ref, err := name.ParseReference(fmt.Sprintf("%s/chainloop-test", b.repo))
	if err != nil {
		return fmt.Errorf("parsing the reference image for validation: %w", err)
	}

	return remote.CheckPushPermission(ref, b.keychain, http.DefaultTransport)
}
