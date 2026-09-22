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

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
)

// stagingFilePrefix names the per-request temporary files, followed by the
// direction. They are unlinked as soon as they are created, so the name never
// shows up in a directory listing; it identifies them in the process's open
// descriptors, where an in-flight request appears as
// "<staging dir>/cas-staging-upload-NNN (deleted)".
const stagingFilePrefix = "cas-staging-"

// stagingKind is the direction of a staged transfer. It names the staging file
// and phrases a digest mismatch for the party that can act on it.
type stagingKind struct {
	name string
	// mismatchMsg explains whose digest the content failed to match
	mismatchMsg string
}

var (
	// stagingUpload: the client declared a digest that does not describe the
	// bytes it sent. The request is invalid.
	stagingUpload = stagingKind{name: "upload", mismatchMsg: "uploaded content does not match the declared digest"}
	// stagingDownload: the backend returned bytes that do not hash to the key
	// they are stored under. The stored content is corrupt or tampered.
	stagingDownload = stagingKind{name: "download", mismatchMsg: "stored content does not match the requested digest"}
)

// stageAndVerify spills whatever fill writes into an unlinked temporary file
// under the staging directory, hashing the bytes in the same pass, and compares
// the result with wantDigest (a hex-encoded SHA256). It is the single integrity
// gate shared by uploads (the producer is the client stream) and downloads (the
// producer is the storage backend), so the paths cannot drift.
//
// On success it returns the file rewound to offset 0 and the number of bytes
// staged; the caller owns the file and must release it with closeStagingFile,
// which frees the space. On any failure the file is already closed and nil is
// returned: a digest mismatch as a *digestMismatchError, a producer failure
// unwrapped for the caller to classify, and staging-disk failures wrapped with
// context.
//
// Staging on disk rather than in memory keeps this service's memory use
// independent of artifact size.
func (s *commonService) stageAndVerify(kind stagingKind, wantDigest string, fill func(w io.Writer) error) (*os.File, int64, error) {
	// os.CreateTemp falls back to the OS temp dir when given an empty path, which
	// on the CAS container is a read-only secret mount. Refuse instead: an
	// unconfigured staging dir is a deployment error, not something to paper over.
	if s.stagingDir == "" {
		return nil, 0, errors.New("no staging directory configured")
	}

	f, err := os.CreateTemp(s.stagingDir, stagingFilePrefix+kind.name+"-*")
	if err != nil {
		return nil, 0, fmt.Errorf("creating staging file: %w", err)
	}

	// From here on, closing the file is the whole cleanup on every exit path.
	fail := func(err error) (*os.File, int64, error) {
		s.closeStagingFile(f)
		return nil, 0, err
	}

	// Drop the directory entry straight away and keep working through the open
	// file descriptor. The staged content stays fully readable and seekable, but
	// it is now owned by this process rather than by the filesystem: the kernel
	// releases the space when the descriptor goes away, including when the
	// process is killed outright.
	//
	// This is what keeps the staging volume bounded. A deferred remove only runs
	// when the handler returns, so a SIGKILL or an OOM kill mid-request would
	// strand a partial artifact on the volume, and nothing would ever reclaim it:
	// the emptyDir backing it outlives container restarts and is cleared only
	// when the Pod is removed from the node. Unlinking up front means an
	// interrupted request cannot leave anything behind, whatever kills us, so no
	// sweep or reaper is needed to keep the volume from filling up.
	if err := os.Remove(f.Name()); err != nil {
		return fail(fmt.Errorf("unlinking staging file: %w", err))
	}

	// Tee the producer into the file and a SHA256 hasher in one pass.
	hasher := sha256.New()
	if err := fill(io.MultiWriter(f, hasher)); err != nil {
		return fail(err)
	}

	// Fail closed: if the bytes do not hash to the expected digest, the file is
	// discarded and nothing downstream ever sees it.
	if got := hex.EncodeToString(hasher.Sum(nil)); got != wantDigest {
		return fail(&digestMismatchError{kind: kind, got: got, want: wantDigest})
	}

	// The write offset is the staged size. Then rewind so the consumer reads from
	// the start. A seekable body also lets the AWS SDK learn the exact length and
	// take its zero-copy SectionReader fast path instead of buffering parts in
	// memory.
	size, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fail(fmt.Errorf("sizing staging file: %w", err))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fail(fmt.Errorf("rewinding staging file: %w", err))
	}

	return f, size, nil
}

// stageDownload stages the backend's copy of digest and verifies it. It is the
// download half of stageAndVerify shared by the gRPC and HTTP handlers, which
// keep only their transport-specific status mapping. A mismatch is logged here,
// once, with the digest the backend actually returned so the operator can
// investigate the backend, and returned as a *digestMismatchError.
func (s *commonService) stageDownload(ctx context.Context, b backend.Downloader, digest string) (*os.File, int64, error) {
	f, size, err := s.stageAndVerify(stagingDownload, digest, func(w io.Writer) error {
		return b.Download(ctx, w, digest)
	})
	if err != nil {
		if mismatch, ok := errors.AsType[*digestMismatchError](err); ok {
			s.log.Warnw("msg", "download rejected: "+stagingDownload.mismatchMsg, "digest", digest, "got", mismatch.got)
		}
		return nil, 0, err
	}

	s.log.Infow("msg", "artifact verified, streaming to client", "digest", digest, "size", size)
	return f, size, nil
}

// closeStagingFile releases a file returned by stageAndVerify. The file is
// already unlinked, so closing it is what frees the space on the staging volume.
func (s *commonService) closeStagingFile(f *os.File) {
	if err := f.Close(); err != nil {
		s.log.Warnw("msg", "failed to close staging file", "error", err.Error())
	}
}

// stagedChunkSize is the size of the gRPC ReadResponse messages a verified
// staging file is streamed in. It stays under grpc-go's largest pooled buffer
// tier (1 MiB) once proto framing is added, so each Send reuses a pooled buffer
// instead of allocating a fresh one.
const stagedChunkSize = 512 << 10

// copyStaged streams a verified staging file to the gRPC client through w in
// stagedChunkSize pieces. The file is deliberately wrapped so its io.WriterTo is
// not picked up by io.CopyBuffer, which would otherwise decide the chunking (and
// therefore the gRPC message size) on its own. The HTTP handler does not use
// this: a plain io.Copy lets the response take the kernel sendfile path.
func copyStaged(w io.Writer, f *os.File) error {
	_, err := io.CopyBuffer(w, struct{ io.Reader }{f}, make([]byte, stagedChunkSize))
	return err
}

// digestMismatchError marks content whose bytes do not hash to the expected
// digest. The kind says whose digest was wrong: on upload the client's declared
// one, on download the backend's stored content. Callers map it to the status
// that fits their direction; no unverified byte is ever forwarded either way.
type digestMismatchError struct {
	kind      stagingKind
	got, want string
}

func (e *digestMismatchError) Error() string {
	return fmt.Sprintf("%s: got=%s, want=%s", e.kind.mismatchMsg, e.got, e.want)
}
