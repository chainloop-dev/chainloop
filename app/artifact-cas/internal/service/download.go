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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	sl "github.com/chainloop-dev/chainloop/internal/servicelogger"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
)

// i.e /download/sha256:1234567890abcdef
const DownloadPath = "/download/{digest}"

type DownloadService struct {
	*commonService
}

func NewDownloadService(bp backend.Provider, opts ...NewOpt) *DownloadService {
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

	hash, err := cr_v1.NewHash(digest)
	if err != nil {
		http.Error(w, "invalid digest", http.StatusBadRequest)
		return
	}

	// Only downloader tokens are allowed
	if err := auth.CheckRole(casJWT.Downloader); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Retrieve the CAS backend from where to download the file
	b, err := s.backendP.FromCredentials(ctx, auth.StoredSecretID)
	if err != nil {
		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	info, err := b.Describe(ctx, hash.Hex)
	if err != nil && backend.IsNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
		return
	}

	// Set headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.FileName))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))

	s.log.Infow("msg", "download initialized", "digest", hash, "size", bytefmt.ByteSize(uint64(info.Size)))

	if err := b.Download(ctx, w, hash.Hex); err != nil {
		if errors.Is(err, context.Canceled) {
			s.log.Infow("msg", "download canceled", "digest", hash)
			return
		}

		http.Error(w, sl.LogAndMaskErr(err, s.log).Error(), http.StatusInternalServerError)
	}

	s.log.Infow("msg", "download finished", "digest", hash, "size", bytefmt.ByteSize(uint64(info.Size)))
}
