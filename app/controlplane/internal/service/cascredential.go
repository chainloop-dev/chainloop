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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	errors "github.com/go-kratos/kratos/v2/errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
)

type CASCredentialsService struct {
	*service
	pb.UnimplementedCASCredentialsServiceServer

	casUC        *biz.CASCredentialsUseCase
	casBackendUC *biz.CASBackendUseCase
	casMappingUC *biz.CASMappingUseCase
	authz        *authz.Enforcer
}

func NewCASCredentialsService(casUC *biz.CASCredentialsUseCase, casmUC *biz.CASMappingUseCase, casBUC *biz.CASBackendUseCase, authz *authz.Enforcer, opts ...NewOpt) *CASCredentialsService {
	return &CASCredentialsService{
		service: newService(opts...),
		casUC:   casUC,
		// we use the casMappingUC to find the backend to download from
		casMappingUC: casmUC,
		// we use the casBackendUC to find the default upload backend
		casBackendUC: casBUC,
		authz:        authz,
	}
}

// Get will generate temporary credentials to be used against the CAS service for the current organization
func (s *CASCredentialsService) Get(ctx context.Context, req *pb.CASCredentialsServiceGetRequest) (*pb.CASCredentialsServiceGetResponse, error) {
	currentUser, currentAPIToken, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	// Load the authz subject from the context
	currentAuthzSubject, err := requireCurrentAuthzSubject(ctx)
	if err != nil {
		return nil, err
	}

	var role casJWT.Role
	var policyToCheck *authz.Policy
	switch req.GetRole() {
	case pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER:
		role = casJWT.Downloader
		policyToCheck = authz.PolicyArtifactDownload
	case pb.CASCredentialsServiceGetRequest_ROLE_UPLOADER:
		role = casJWT.Uploader
		policyToCheck = authz.PolicyArtifactUpload
	}

	if ok, err := s.authz.Enforce(currentAuthzSubject, policyToCheck); err != nil {
		return nil, handleUseCaseErr(err, s.log)
	} else if !ok {
		return nil, errors.Forbidden("forbidden", "not allowed to perform this operation")
	}

	// Load the default CAS backend, we'll use it for uploads and as fallback on downloads
	backend, err := s.casBackendUC.FindDefaultBackend(ctx, currentOrg.ID)
	if err != nil && !biz.IsNotFound(err) {
		return nil, handleUseCaseErr(err, s.log)
	} else if backend == nil {
		return nil, errors.NotFound("not found", "main CAS backend not found")
	}

	// Try to find the proper backend where the artifact is stored
	if role == casJWT.Downloader {
		var mapping *biz.CASMapping
		// If we are logged in as a user, we'll try to find a mapping for that user
		if currentUser != nil {
			mapping, err = s.casMappingUC.FindCASMappingForDownloadByUser(ctx, req.Digest, currentUser.ID)
			// otherwise, we'll try to find a mapping for the current API token associated orgs
		} else if currentAPIToken != nil {
			mapping, err = s.casMappingUC.FindCASMappingForDownloadByOrg(ctx, req.Digest, []string{currentOrg.ID})
		}

		// If we can't find a mapping, we'll use the default backend
		if err != nil && !biz.IsNotFound(err) && !biz.IsErrUnauthorized(err) {
			if biz.IsErrValidation(err) {
				return nil, errors.BadRequest("invalid", err.Error())
			}

			return nil, handleUseCaseErr(err, s.log)
		}

		if mapping != nil {
			backend = mapping.CASBackend
		}
	}

	// inline backends don't have a download URL
	if backend.Inline {
		return nil, errors.BadRequest("invalid argument", "cannot upload or download artifacts from an inline CAS backend")
	}

	ref := &biz.CASCredsOpts{BackendType: string(backend.Provider), SecretPath: backend.SecretName, Role: role, MaxBytes: backend.Limits.MaxBytes}
	t, err := s.casUC.GenerateTemporaryCredentials(ref)
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	return &pb.CASCredentialsServiceGetResponse{
		Result: &pb.CASCredentialsServiceGetResponse_Result{Token: t},
	}, nil
}
