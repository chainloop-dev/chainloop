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

type ConfigCurrentContext struct {
	cfg *ActionsOpts
}

func NewConfigCurrentContext(cfg *ActionsOpts) *ConfigCurrentContext {
	return &ConfigCurrentContext{cfg}
}

type ConfigContextItem struct {
	CurrentUser       *UserItem
	CurrentMembership *MembershipItem
	CurrentCASBackend *CASBackendItem
}

type UserItem struct {
	ID, Email, FirstName, LastName string
	CreatedAt                      *time.Time
}

// PrintUserProfileWithEmail formats the user's profile with their email.
func (u *UserItem) PrintUserProfileWithEmail() string {
	var name string

	// Build name based on available parts
	if u.FirstName != "" && u.LastName != "" {
		name = u.FirstName + " " + u.LastName
	} else if u.FirstName != "" {
		name = u.FirstName
	} else if u.LastName != "" {
		name = u.LastName
	}

	// If we have a name, format with email, otherwise just return email
	if name != "" {
		return name + " <" + u.Email + ">"
	}
	return u.Email
}

func (action *ConfigCurrentContext) Run() (*ConfigContextItem, error) {
	client := pb.NewContextServiceClient(action.cfg.CPConnection)
	resp, err := client.Current(context.Background(), &pb.ContextServiceCurrentRequest{})
	if err != nil {
		return nil, err
	}

	res := resp.GetResult()

	return &ConfigContextItem{
		CurrentUser:       pbUserItemToAction(res.GetCurrentUser()),
		CurrentMembership: pbMembershipItemToAction(res.GetCurrentMembership()),
		CurrentCASBackend: pbCASBackendItemToAction(res.GetCurrentCasBackend()),
	}, nil
}

func pbUserItemToAction(in *pb.User) *UserItem {
	if in == nil {
		return nil
	}

	return &UserItem{
		ID:        in.Id,
		Email:     in.Email,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		CreatedAt: toTimePtr(in.CreatedAt.AsTime()),
	}
}
