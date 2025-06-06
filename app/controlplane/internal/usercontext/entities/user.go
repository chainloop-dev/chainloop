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

// Utils to get and set information from context
type User struct {
	Email, ID, FirstName, LastName string
	CreatedAt                      *time.Time
}

func WithCurrentUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, currentUserCtxKey{}, user)
}

// RequestID tries to retrieve requestID from the given context.
// If it doesn't exist, an empty string is returned.
func CurrentUser(ctx context.Context) *User {
	res := ctx.Value(currentUserCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*User)
}

type currentUserCtxKey struct{}
