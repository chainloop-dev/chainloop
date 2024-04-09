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

type CASBackendList struct {
	cfg *ActionsOpts
}

type CASBackendItem struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Location         string            `json:"location"`
	Description      string            `json:"description"`
	Provider         string            `json:"provider"`
	Default          bool              `json:"default"`
	Inline           bool              `json:"inline"`
	Limits           *CASBackendLimits `json:"limits"`
	ValidationStatus ValidationStatus  `json:"validationStatus"`

	CreatedAt   *time.Time `json:"createdAt"`
	ValidatedAt *time.Time `json:"validatedAt"`
}

type CASBackendLimits struct {
	// Max number of bytes allowed to be stored in this backend
	MaxBytes int64
}

type ValidationStatus string

const (
	Valid   ValidationStatus = "valid"
	Invalid ValidationStatus = "invalid"
)

func NewCASBackendList(cfg *ActionsOpts) *CASBackendList {
	return &CASBackendList{cfg}
}

func (action *CASBackendList) Run() ([]*CASBackendItem, error) {
	client := pb.NewCASBackendServiceClient(action.cfg.CPConnection)
	resp, err := client.List(context.Background(), &pb.CASBackendServiceListRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*CASBackendItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbCASBackendItemToAction(p))
	}

	return result, nil
}

func pbCASBackendItemToAction(in *pb.CASBackendItem) *CASBackendItem {
	if in == nil {
		return nil
	}

	b := &CASBackendItem{
		ID:          in.Id,
		Name:        in.Name,
		Location:    in.Location,
		Description: in.Description,
		Provider:    in.Provider,
		Default:     in.Default,
		CreatedAt:   toTimePtr(in.CreatedAt.AsTime()),
		ValidatedAt: toTimePtr(in.ValidatedAt.AsTime()),
		Inline:      in.IsInline,
	}

	if in.Limits != nil {
		b.Limits = &CASBackendLimits{
			MaxBytes: in.Limits.MaxBytes,
		}
	}

	switch in.GetValidationStatus() {
	case pb.CASBackendItem_VALIDATION_STATUS_OK:
		b.ValidationStatus = Valid
	case pb.CASBackendItem_VALIDATION_STATUS_INVALID:
		b.ValidationStatus = Invalid
	}

	return b
}
