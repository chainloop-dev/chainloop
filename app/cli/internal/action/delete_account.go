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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type DeleteAccount struct {
	*ActionsOpts
}

func NewDeleteAccount(cfg *ActionsOpts) *DeleteAccount {
	return &DeleteAccount{cfg}
}

func (a *DeleteAccount) Run() error {
	client := pb.NewAuthServiceClient(a.CPConnecction)
	_, err := client.DeleteAccount(context.Background(), &pb.AuthServiceDeleteAccountRequest{})
	return err
}
