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

package usercontext

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
)

// GetRawToken takes whatever Bearer token is in the request
func GetRawToken(ctx context.Context) (string, error) {
	header, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", jwt.ErrMissingJwtToken
	}

	auths := strings.SplitN(header.RequestHeader().Get("Authorization"), " ", 2)
	if len(auths) != 2 || !strings.EqualFold(auths[0], "Bearer") {
		return "", jwt.ErrMissingJwtToken
	}
	return auths[1], nil
}
