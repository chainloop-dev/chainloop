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

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	sl "github.com/chainloop-dev/chainloop/pkg/servicelogger"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
)

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
	auth, err := infoFromAuth(ctx)
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

	// if the buffer contains the actual data we expect we proceed with sending it to the browser
	// Set headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.FileName))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	s.log.Infow("msg", "download initialized", "digest", wantChecksum, "size", bytefmt.ByteSize(uint64(info.Size)))

	gotChecksum := sha256.New()
	// create temporary buffer to write to both the writer and the checksum
	buf := bytes.NewBuffer(nil)

	// NOTE: we don't sent the file directly to the writer because we need to calculate the checksum
	// and we want to send the file / even if partially only if the checksum matches
	// this has a performance impact but it's the only way to ensure that the file is not corrupted
	// and don't require client-side verification
	mw := io.MultiWriter(buf, gotChecksum)
	if err := b.Download(ctx, mw, wantChecksum.Hex); err != nil {
		if errors.Is(err, context.Canceled) {
			s.log.Infow("msg", "download canceled", "digest", wantChecksum)
			return
		}

		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	// Verify the checksum
	if got, want := fmt.Sprintf("%x", gotChecksum.Sum(nil)), wantChecksum.Hex; got != want {
		msg := fmt.Sprintf("checksums mismatch: got: %s, want: %s", got, want)
		s.log.Info(msg)
		http.Error(w, msg, http.StatusUnauthorized)
		return
	}

	if _, err := io.Copy(w, buf); err != nil {
		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	s.log.Infow("msg", "download finished", "digest", wantChecksum, "size", bytefmt.ByteSize(uint64(info.Size)))
}
