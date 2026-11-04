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
	"context"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ReferrerService struct {
	pb.UnimplementedReferrerServiceServer
	*service

	referrerUC *biz.ReferrerUseCase
}

func NewReferrerService(uc *biz.ReferrerUseCase, opts ...NewOpt) *ReferrerService {
	return &ReferrerService{
		service:    newService(opts...),
		referrerUC: uc,
	}
}

func (s *ReferrerService) DiscoverPrivate(ctx context.Context, req *pb.ReferrerServiceDiscoverPrivateRequest) (*pb.ReferrerServiceDiscoverPrivateResponse, error) {
	if _, _, err := requireCurrentUserOrAPIToken(ctx); err != nil {
		return nil, err
	}

	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	paginationOpts, err := referrerPaginationOptsFromProto(req.GetPagination())
	if err != nil {
		return nil, err
	}

	// Optionally scope the discovery by project, optionally narrowed to a project version.
	// project_version requires project_name (enforced at the proto validation layer); a
	// project_name on its own scopes to every version of that project.
	var extraFilters []biz.GetFromRootFilter
	if req.GetProjectName() != "" {
		extraFilters = append(extraFilters, biz.WithProjectScope(req.GetProjectName(), req.GetProjectVersion()))
	}

	orgUUID, err := uuid.Parse(currentOrg.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid org UUID: %w", err)
	}

	// Discovery is always limited to the current organization, with the project visibility
	// of the caller, the same way for users and API tokens
	referrer, nextCursor, err := s.referrerUC.GetFromRoot(ctx, req.GetDigest(), req.GetKind(), []uuid.UUID{orgUUID},
		s.rbacScopesForOrg(ctx, orgUUID).ProjectIDsByOrg(), paginationOpts, extraFilters...)
	if err != nil {
		return nil, handleUseCaseErr(ctx, err, s.log)
	}

	return &pb.ReferrerServiceDiscoverPrivateResponse{
		Result:     bizReferrerToPb(referrer),
		Pagination: bizCursorToPb(nextCursor),
		// Tell the client that the search covered only the current organization,
		// so that it can reject the cross-organization results of older control planes
		CurrentOrgScoped: true,
	}, nil
}

// defaultReferrerPageSize is the page size applied when a referrer Discover* request
// arrives without pagination. It deliberately overrides the package-wide
// pagination.DefaultCursorLimit (10) because referrer responses render nested references
// (SBOMs, SARIF, …) and a slightly larger page is easier to navigate without being
// noticeably slower. Keep ≤ the proto-enforced max of 100.
const defaultReferrerPageSize = 20

// referrerPaginationOptsFromProto converts the proto pagination request to cursor options.
// When the request has no pagination or an unset limit, the default page size is applied
// so that the response is always bounded. A root referrer (e.g. a container image) can
// accumulate an unbounded number of direct references (SBOMs, SARIF reports, ...) as it
// is attested repeatedly — returning all of them in a single response is unsafe.
func referrerPaginationOptsFromProto(p *pb.CursorPaginationRequest) (*pagination.CursorOptions, error) {
	limit := defaultReferrerPageSize
	var cursor string
	if p != nil {
		cursor = p.GetCursor()
		if l := int(p.GetLimit()); l > 0 {
			limit = l
		}
	}
	return pagination.NewCursor(cursor, limit)
}

func bizReferrerToPb(r *biz.StoredReferrer) *pb.ReferrerItem {
	item := &pb.ReferrerItem{
		Digest:       r.Digest,
		Downloadable: r.Downloadable,
		Kind:         r.Kind,
		CreatedAt:    timestamppb.New(*r.CreatedAt),
		Metadata:     r.Metadata,
		Annotations:  r.Annotations,
	}

	for _, r := range r.References {
		item.References = append(item.References, bizReferrerToPb(r))
	}

	return item
}
