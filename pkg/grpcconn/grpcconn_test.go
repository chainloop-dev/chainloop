//
// Copyright 2024-2025 The Chainloop Authors.
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

package grpcconn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRequestMetadata(t *testing.T) {
	const wantOrg = "org-1"
	want := map[string]string{"authorization": "Bearer token", "Chainloop-Organization": wantOrg}
	auth := newTokenAuth("token", false, wantOrg)
	got, err := auth.GetRequestMetadata(context.TODO())
	assert.NoError(t, err)

	assert.Equal(t, got, want)
}

func TestRequireTransportSecurity(t *testing.T) {
	testCases := []struct {
		insecure bool
		want     bool
	}{
		{insecure: true, want: false},
		{insecure: false, want: true},
	}

	for _, tc := range testCases {
		auth := newTokenAuth("token", tc.insecure, "org-1")
		assert.Equal(t, tc.want, auth.RequireTransportSecurity())
	}
}
