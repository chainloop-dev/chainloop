//
// Copyright 2024-2025 The Chainloop Authors.
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
	"context"
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type APITokenList struct {
	cfg *ActionsOpts
}

func NewAPITokenList(cfg *ActionsOpts) *APITokenList {
	return &APITokenList{cfg}
}

func (action *APITokenList) Run(ctx context.Context, includeRevoked bool, project string, scope string) ([]*APITokenItem, error) {
	client := pb.NewAPITokenServiceClient(action.cfg.CPConnection)

	req := &pb.APITokenServiceListRequest{IncludeRevoked: includeRevoked}
	if project != "" {
		req.Project = &pb.IdentityReference{Name: &project}
	}

	if scope != "" {
		req.Scope = mapScope(scope)
	}

	resp, err := client.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("listing API tokens: %w", err)
	}

	result := make([]*APITokenItem, 0, len(resp.Result))
	for _, t := range resp.Result {
		result = append(result, pbAPITokenItemToAPITokenItem(t))
	}

	return result, nil
}

func mapScope(scope string) pb.APITokenServiceListRequest_Scope {
	switch scope {
	case "project":
		return pb.APITokenServiceListRequest_SCOPE_PROJECT
	case "global":
		return pb.APITokenServiceListRequest_SCOPE_GLOBAL
	default:
		return pb.APITokenServiceListRequest_SCOPE_UNSPECIFIED
	}
}
