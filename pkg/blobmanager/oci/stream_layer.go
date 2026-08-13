//
// Copyright 2026 The Chainloop Authors.
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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync/atomic"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// rawStreamLayer is a v1.Layer implementation for content-addressed raw
// (uncompressed) blobs whose digest and size are known up front. It lets the
// artifact bytes be streamed straight to the registry blob endpoint without
// buffering the whole content in memory.
//
// go-containerregistry's remote.Write passes the value returned by Compressed()
// directly as the HTTP request body (no io.ReadAll), so a Compressed() that
// yields the raw stream streams the artifact through with bounded memory. The
// provided stream.Layer cannot be used here because it gzip-compresses the
// content and only computes the digest after the stream is consumed, which is
// incompatible with storing raw, content-addressed data.
//
// Because the content is stored uncompressed, the layer Digest and DiffID are
// identical; mutate embeds the DiffID into the image config's rootfs.diff_ids,
// keeping the existing download path (remote.Image().LayerByDiffID) intact.
//
// The underlying stream can be read only once. Compressed/Uncompressed return an
// error on any subsequent call so that a retry or an HTTP/2 body replay fails
// cleanly instead of silently uploading a truncated, already-drained reader.
type rawStreamLayer struct {
	hash      v1.Hash
	size      int64
	mediaType types.MediaType
	body      io.ReadCloser
	consumed  atomic.Bool
}

// Digest returns the sha256 of the raw content, known up front.
func (l *rawStreamLayer) Digest() (v1.Hash, error) { return l.hash, nil }

// DiffID equals Digest because the content is stored uncompressed.
func (l *rawStreamLayer) DiffID() (v1.Hash, error) { return l.hash, nil }

// Size returns the raw content size in bytes, known up front.
func (l *rawStreamLayer) Size() (int64, error) { return l.size, nil }

// MediaType returns the detected media type of the raw content.
func (l *rawStreamLayer) MediaType() (types.MediaType, error) { return l.mediaType, nil }

// Compressed returns the raw content stream. The content is not compressed; the
// name follows the v1.Layer interface, and remote.Write sends these bytes to the
// registry verbatim.
func (l *rawStreamLayer) Compressed() (io.ReadCloser, error) { return l.reader() }

// Uncompressed returns the same raw content stream as Compressed.
func (l *rawStreamLayer) Uncompressed() (io.ReadCloser, error) { return l.reader() }

func (l *rawStreamLayer) reader() (io.ReadCloser, error) {
	if l.consumed.Swap(true) {
		return nil, errors.New("raw stream layer body already consumed: a streamed upload cannot be retried on a non-replayable reader")
	}
	return l.body, nil
}

// verifyingReader streams r while computing its sha256 over the first size bytes
// and fails if the digest does not match wantHex. It restores the
// content-integrity check the buffered OCI upload path performed (static.NewLayer
// computed the layer digest from the bytes, which validateImage then compared to
// the declared digest) without buffering the artifact: a mismatch makes Read
// return an error, which aborts remote.Write before the blob is committed.
//
// The check fires as soon as size bytes have been read, not only at EOF: the
// HTTP transport uploads exactly Content-Length (size) bytes and may stop
// reading there without ever pulling the trailing EOF, so an EOF-only check
// could be skipped entirely before the registry commit.
type verifyingReader struct {
	r         io.Reader
	hasher    hash.Hash
	wantHex   string
	remaining int64
	verified  bool
}

func newVerifyingReader(r io.Reader, wantHex string, size int64) *verifyingReader {
	return &verifyingReader{r: r, hasher: sha256.New(), wantHex: wantHex, remaining: size}
}

func (v *verifyingReader) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	if n > 0 && v.remaining > 0 {
		// Hash at most the declared number of bytes: only those are uploaded.
		h := int64(n)
		if h > v.remaining {
			h = v.remaining
		}
		// hash.Hash never returns an error.
		_, _ = v.hasher.Write(p[:h])
		v.remaining -= h
	}
	// Verify once the declared byte count is reached or at EOF, whichever comes
	// first.
	if !v.verified && (v.remaining <= 0 || errors.Is(err, io.EOF)) {
		v.verified = true
		if got := hex.EncodeToString(v.hasher.Sum(nil)); got != v.wantHex {
			return n, fmt.Errorf("content digest mismatch: got sha256:%s, want sha256:%s", got, v.wantHex)
		}
	}
	return n, err
}
