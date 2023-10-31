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

type OrgInviteListSent struct {
	cfg *ActionsOpts
}

type OrgInviteItem struct {
	ID            string     `json:"id"`
	ReceiverEmail string     `json:"receiverEmail"`
	Organization  *OrgItem   `json:"organization"`
	Sender        *UserItem  `json:"sender"`
	Status        string     `json:"status"`
	CreatedAt     *time.Time `json:"createdAt"`
}

func NewOrgInviteListSent(cfg *ActionsOpts) *OrgInviteListSent {
	return &OrgInviteListSent{cfg}
}

func (action *OrgInviteListSent) Run(ctx context.Context) ([]*OrgInviteItem, error) {
	client := pb.NewOrgInviteServiceClient(action.cfg.CPConnection)
	resp, err := client.ListSent(ctx, &pb.OrgInviteServiceListSentRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*OrgInviteItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbOrgInviteItemToAction(p))
	}

	return result, nil
}

func pbOrgInviteItemToAction(in *pb.OrgInviteItem) *OrgInviteItem {
	if in == nil {
		return nil
	}

	return &OrgInviteItem{
		ID:            in.Id,
		ReceiverEmail: in.ReceiverEmail,
		Organization:  pbOrgItemToAction(in.Organization),
		Sender:        pbUserItemToAction(in.Sender),
		CreatedAt:     toTimePtr(in.CreatedAt.AsTime()),
		Status:        in.Status,
	}
}
