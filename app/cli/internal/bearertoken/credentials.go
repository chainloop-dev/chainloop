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

package bearertoken

import (
	"context"

	"google.golang.org/grpc/credentials"
)

type TokenAuth struct {
	token    string
	insecure bool
}

// Implementation of PerRPCCredentials interface that sends a bearer token in each request.
func NewTokenAuth(token string, insecure bool) credentials.PerRPCCredentials {
	return &TokenAuth{token, insecure}
}

// Return value is mapped to request headers.
func (t TokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t TokenAuth) RequireTransportSecurity() bool {
	return !t.insecure
}
