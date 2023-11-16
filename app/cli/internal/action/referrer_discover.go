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

package action

import (
	"context"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type ReferrerDiscover struct {
	cfg *ActionsOpts
}
type ReferrerDiscoverPublic struct {
	cfg *ActionsOpts
}

type ReferrerItem struct {
	Digest       string          `json:"digest"`
	Kind         string          `json:"kind"`
	Downloadable bool            `json:"downloadable"`
	Public       bool            `json:"public"`
	CreatedAt    *time.Time      `json:"createdAt"`
	References   []*ReferrerItem `json:"references"`
}

func NewReferrerDiscoverPrivate(cfg *ActionsOpts) *ReferrerDiscover {
	return &ReferrerDiscover{cfg}
}

func (action *ReferrerDiscover) Run(ctx context.Context, digest, kind string) (*ReferrerItem, error) {
	client := pb.NewReferrerServiceClient(action.cfg.CPConnection)
	resp, err := client.DiscoverPrivate(ctx, &pb.ReferrerServiceDiscoverPrivateRequest{
		Digest: digest, Kind: kind,
	})
	if err != nil {
		return nil, err
	}

	return pbReferrerItemToAction(resp.Result), nil
}

func NewReferrerDiscoverPublicIndex(cfg *ActionsOpts) *ReferrerDiscoverPublic {
	return &ReferrerDiscoverPublic{cfg}
}

func (action *ReferrerDiscoverPublic) Run(ctx context.Context, digest, kind string) (*ReferrerItem, error) {
	client := pb.NewReferrerServiceClient(action.cfg.CPConnection)
	resp, err := client.DiscoverPublicShared(ctx, &pb.DiscoverPublicSharedRequest{
		Digest: digest, Kind: kind,
	})
	if err != nil {
		return nil, err
	}

	return pbReferrerItemToAction(resp.Result), nil
}

func pbReferrerItemToAction(in *pb.ReferrerItem) *ReferrerItem {
	if in == nil {
		return nil
	}

	out := &ReferrerItem{
		Digest:       in.GetDigest(),
		Downloadable: in.GetDownloadable(),
		Public:       in.GetPublic(),
		Kind:         in.GetKind(),
		CreatedAt:    toTimePtr(in.GetCreatedAt().AsTime()),
		References:   make([]*ReferrerItem, 0, len(in.GetReferences())),
	}

	for _, r := range in.GetReferences() {
		out.References = append(out.References, pbReferrerItemToAction(r))
	}

	return out
}
