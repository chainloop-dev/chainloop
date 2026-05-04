//
// Copyright 2025-2026 The Chainloop Authors.
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
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/grpcconn"
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

func GetOrganizationNameFromHeader(ctx context.Context) (string, error) {
	const OrganizationHeader = "Chainloop-Organization"
	header, ok := transport.FromServerContext(ctx)
	if ok {
		return header.RequestHeader().Get(OrganizationHeader), nil
	}

	return "", nil
}

// GetCLIVersionFromHeader returns the CLI version advertised by the caller in
// the Chainloop-Cli-Version request header. The value format is
// "<version>-<edition>", e.g. "v1.94.2-oss". Returns an empty string when the
// header is absent or there is no transport in the context.
func GetCLIVersionFromHeader(ctx context.Context) string {
	header, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return header.RequestHeader().Get(grpcconn.CLIVersionHeader)
}
