//
// Copyright 2024 The Chainloop Authors.
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

package entities

import (
	"context"
	"time"
)

type Org struct {
	ID, Name  string
	CreatedAt *time.Time
}

func WithCurrentOrg(ctx context.Context, org *Org) context.Context {
	return context.WithValue(ctx, currentOrgCtxKey{}, org)
}

func CurrentOrg(ctx context.Context) *Org {
	res := ctx.Value(currentOrgCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*Org)
}

type currentOrgCtxKey struct{}
