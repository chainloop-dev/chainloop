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

package oci

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

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
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
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

func NewBackend(repository string, regOpts *RegistryOptions, opts ...NewBackendOpt) (*Backend, error) {
	b := &Backend{
		repo:     repository,
		prefix:   backend.DefaultPrefix,
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

// sniffLen is the number of leading bytes http.DetectContentType inspects to
// determine the media type. We peek exactly this many bytes off the stream so
// the media type is detected identically to the buffered path without reading
// the whole artifact into memory.
const sniffLen = 512

// SupportsStreaming reports that the OCI backend can upload directly from a
// streaming reader. Streaming requires the artifact size to be known up front
// (registry blobs are pushed with a fixed Content-Length); when the client did
// not provide it Upload transparently falls back to buffering the content.
func (b *Backend) SupportsStreaming() bool { return true }

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	ref, err := name.ParseReference(b.resourcePath(resource.Digest))
	if err != nil {
		return fmt.Errorf("parsing reference: %w", err)
	}

	// The image is either backed by a streaming layer (when the size is known)
	// or a fully buffered one (legacy clients that do not send the size).
	img, err := b.craftImageFromReader(r, resource)
	if err != nil {
		return fmt.Errorf("crafting image: %w", err)
	}

	if err := validateImage(img, resource.Digest); err != nil {
		return fmt.Errorf("validating image: %w", err)
	}

	// Disable go-containerregistry's request retries: a streamed layer body
	// cannot be replayed, so a retry would re-request the already-drained reader
	// and upload truncated content. On a transient failure the whole upload is
	// retried by the client instead. The retry-disable is harmless for the
	// buffered path too.
	if err := remote.Write(ref, img,
		remote.WithAuthFromKeychain(b.keychain),
		remote.WithContext(ctx),
		remote.WithRetryPredicate(func(error) bool { return false }),
	); err != nil {
		return fmt.Errorf("writing image: %w", err)
	}

	return nil
}

// craftImageFromReader builds the chainloop OCI image around the artifact
// content. When the artifact size is known (resource.Size > 0) the layer is
// streamed straight from r, keeping memory bounded regardless of artifact size.
// Otherwise it falls back to buffering the whole content, which is
// required for legacy clients that do not report the size (an empty artifact has
// size 0 and is rejected by the buffered path).
func (b *Backend) craftImageFromReader(r io.Reader, resource *pb.CASResource) (v1.Image, error) {
	if resource == nil || resource.Digest == "" {
		return nil, errors.New("resource metadata is not valid")
	}

	if resource.Size <= 0 {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("reading content: %w", err)
		}
		return craftImage(data, resource)
	}

	return craftStreamingImage(r, resource)
}

func (b *Backend) resourcePath(resourceName string) string {
	return fmt.Sprintf("%s/%s-%s", b.repo, b.prefix, resourceName)
}

// craftStreamingImage assembles the image with a layer whose bytes are streamed
// from r. The media type is detected from the leading bytes, which are peeked
// (not consumed) so the streamed content is complete.
func craftStreamingImage(r io.Reader, resource *pb.CASResource) (v1.Image, error) {
	hash, err := v1.NewHash("sha256:" + resource.Digest)
	if err != nil {
		return nil, fmt.Errorf("parsing digest: %w", err)
	}

	// Peek the leading bytes to detect the media type without consuming them.
	br := bufio.NewReaderSize(r, sniffLen)
	head, err := br.Peek(sniffLen)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, fmt.Errorf("detecting media type: %w", err)
	}

	// Verify the streamed content against the declared digest as it flows to the
	// registry. The buffered path got this for free (the layer digest was
	// computed from the bytes); here rawStreamLayer.Digest() returns the declared
	// digest, so without this a client could store content under a mismatched key
	// on a registry that does not strictly verify blob digests on commit.
	layer := &rawStreamLayer{
		hash:      hash,
		size:      resource.Size,
		mediaType: backend.DetectedMediaType(head),
		body:      io.NopCloser(newVerifyingReader(br, resource.Digest, resource.Size)),
	}

	return buildImage(layer, resource)
}

// craftImage builds the image from the whole content buffered in memory. Kept
// for legacy clients that do not report the artifact size up front.
func craftImage(content []byte, resource *pb.CASResource) (v1.Image, error) {
	if len(content) == 0 {
		return nil, errors.New("content is empty")
	}

	layer := static.NewLayer(content, backend.DetectedMediaType(content))
	return buildImage(layer, resource)
}

// buildImage assembles the chainloop OCI image (OCI manifest + OCI config + a
// single raw layer + authors/title annotations) around an already-constructed
// layer. Using an OCIConfigJSON config makes mutate populate the config's
// rootfs.diff_ids from the layer DiffID, which the download path relies on
// (remote.Image().LayerByDiffID).
func buildImage(layer v1.Layer, resource *pb.CASResource) (v1.Image, error) {
	if resource == nil || resource.FileName == "" || resource.Digest == "" {
		return nil, errors.New("resource metadata is not valid")
	}

	base := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	base = mutate.ConfigMediaType(base, types.OCIConfigJSON)
	base = mutate.Annotations(base, map[string]string{
		ocispec.AnnotationAuthors: backend.AuthorAnnotation,
		// TODO: Move this annotation to layer
		ocispec.AnnotationTitle: resource.FileName,
	}).(v1.Image)

	img, err := mutate.Append(base, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, err
	}

	return img, nil
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

	if v, ok := m.Annotations[ocispec.AnnotationAuthors]; !ok || v != backend.AuthorAnnotation {
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
