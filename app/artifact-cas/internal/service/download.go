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

package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
)

var downloadTracer = otelx.Tracer("chainloop-cas", "cas/service/download")

// i.e /download/sha256:1234567890abcdef
const DownloadPath = "/download/{digest}"

type DownloadService struct {
	*commonService
}

func NewDownloadService(bp backend.Providers, opts ...NewOpt) *DownloadService {
	return &DownloadService{
		commonService: newCommonService(bp, opts...),
	}
}

func (s *DownloadService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := otelx.Start(ctx, downloadTracer, "DownloadService.ServeHTTP")
	defer span.End()
	auth, err := casJWT.InfoFromAuth(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	digest, ok := mux.Vars(r)["digest"]
	if !ok {
		http.Error(w, "missing digest", http.StatusBadRequest)
		return
	}

	wantChecksum, err := cr_v1.NewHash(digest)
	if err != nil {
		http.Error(w, "invalid digest", http.StatusBadRequest)
		return
	}

	// Only downloader tokens are allowed
	if err := auth.CheckRole(casJWT.Downloader); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	b, err := s.loadBackend(ctx, auth.BackendType, auth.StoredSecretID)
	if err != nil && kerrors.IsNotFound(err) {
		http.Error(w, "backend not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	info, err := b.Describe(ctx, wantChecksum.Hex)
	if err != nil && backend.IsNotFound(err) {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	// Override file nane if one is provided
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = info.FileName
	}
	s.log.Infow("msg", "download initialized", "digest", wantChecksum, "size", bytefmt.ByteSize(uint64(info.Size)))

	// Stage the backend egress on local disk and verify it against the requested
	// digest before anything is written to the response: the browser never
	// receives content that does not hash to the digest it asked for, and CAS
	// memory stays bounded because the artifact lives on disk, not in a buffer.
	f, size, err := s.stageDownload(ctx, b, wantChecksum.Hex)
	if err != nil {
		s.writeDownloadError(w, err, wantChecksum.Hex)
		return
	}
	defer s.closeStagingFile(f)

	// The content is verified: announce it to the browser with its exact size
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

	// A plain io.Copy lets the response writer pull the file with sendfile, so
	// the verified bytes go kernel-to-kernel without a user-space buffer.
	if _, err := io.Copy(w, f); err != nil {
		s.writeDownloadError(w, err, wantChecksum.Hex)
		return
	}

	s.log.Infow("msg", "download finished", "digest", wantChecksum, "size", bytefmt.ByteSize(uint64(size)))
	s.audit.Dispatch(&events.CASArtifactDownloaded{
		CASArtifactBase: &events.CASArtifactBase{
			Digest:      wantChecksum.Hex,
			SizeBytes:   size,
			FileName:    filename,
			BackendType: auth.BackendType,
		},
	}, auth)
}

// writeDownloadError maps a staging or streaming failure of a download to its
// HTTP response. A mismatch means the backend holds content that does not hash
// to its key, corrupt or tampered, so it is a 500 carrying both digests rather
// than blamed on the caller. A client that went away gets nothing. Anything
// else is masked.
func (s *DownloadService) writeDownloadError(w http.ResponseWriter, err error, digest string) {
	if _, ok := errors.AsType[*digestMismatchError](err); ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isClientDisconnect(err) {
		s.log.Infow("msg", "download canceled", "digest", digest)
		return
	}

	http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
}
