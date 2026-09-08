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

package action

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

// traceProjectPageSize is how many projects are fetched per request. It is the
// maximum the control plane accepts, so most organizations need a single call.
const traceProjectPageSize int32 = 100

// ListOrganizations returns the organizations the current user belongs to, so
// `trace init` can offer them. It is a UserService call, which ignores the
// organization header, and so returns the same list on any connection.
func (e *AttestationExecutor) ListOrganizations(ctx context.Context) ([]*MembershipItem, error) {
	return NewMembershipList(e.actionOpts).ListOrgs(ctx)
}

// ListProjects returns the projects visible in the organization the executor is
// pinned to, sorted by name and deduplicated. It runs over the executor's
// connection, so it targets the organization pinned with WithForcedOrganization
// like every other trace operation.
func (e *AttestationExecutor) ListProjects(ctx context.Context) ([]*TraceProject, error) {
	return listAllTraceProjects(ctx, &cpTraceProjectAPI{cfg: e.actionOpts})
}

// TraceProject is one entry of the project listing.
type TraceProject struct {
	Name string
	// CanCreateWorkflow is false for a project the caller can see but not add a
	// workflow to, such as one they only view. Offering one of those would end
	// in a permission error the moment init tried to create the workflow.
	CanCreateWorkflow bool
}

// traceProjectAPI is the slice of the control-plane API the project listing
// drives, so the paging can be tested without a live control plane.
type traceProjectAPI interface {
	// listProjectsPage returns one page as the server sent it, plus the total
	// number of pages it reports.
	listProjectsPage(ctx context.Context, page, pageSize int32) ([]*TraceProject, int32, error)
}

// listAllTraceProjects walks every page and returns what the caller can see.
// Deciding which of those to offer is left to the caller, which needs to tell a
// project it may not write to apart from one that is missing entirely.
func listAllTraceProjects(ctx context.Context, api traceProjectAPI) ([]*TraceProject, error) {
	projects := make([]*TraceProject, 0, traceProjectPageSize)

	for page := int32(1); ; page++ {
		got, totalPages, err := api.listProjectsPage(ctx, page, traceProjectPageSize)
		if err != nil {
			return nil, fmt.Errorf("listing projects: %w", err)
		}

		projects = append(projects, got...)

		// Stop on the last page the server reports, and also on a page the server
		// returned nothing for, so a server reporting more pages than it serves
		// cannot spin here.
		if len(got) == 0 || page >= totalPages {
			break
		}
	}

	slices.SortFunc(projects, func(a, b *TraceProject) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return slices.CompactFunc(projects, func(a, b *TraceProject) bool {
		return a.Name == b.Name
	}), nil
}

// cpTraceProjectAPI implements traceProjectAPI against the control plane.
type cpTraceProjectAPI struct {
	cfg *ActionsOpts
}

func (a *cpTraceProjectAPI) listProjectsPage(ctx context.Context, page, pageSize int32) ([]*TraceProject, int32, error) {
	client := pb.NewProjectServiceClient(a.cfg.CPConnection)

	resp, err := client.List(ctx, &pb.ProjectServiceListRequest{
		Pagination: &pb.OffsetPaginationRequest{Page: page, PageSize: pageSize},
	})
	if err != nil {
		return nil, 0, err
	}

	projects := make([]*TraceProject, 0, len(resp.GetProjects()))
	for _, p := range resp.GetProjects() {
		projects = append(projects, &TraceProject{
			Name:              p.GetName(),
			CanCreateWorkflow: p.GetCanCreateWorkflow(),
		})
	}

	return projects, resp.GetPagination().GetTotalPages(), nil
}
