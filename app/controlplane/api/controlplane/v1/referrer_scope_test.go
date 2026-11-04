//
// Copyright 2026 The Chainloop Authors.
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

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func TestDiscoverScopeUnaryClientInterceptor(t *testing.T) {
	errInvoke := errors.New("invoke failed")

	testCases := []struct {
		name      string
		reply     any
		serverOut proto.Message
		invokeErr error
		wantErr   error
	}{
		{
			name:      "discover response that reports the scope",
			reply:     &ReferrerServiceDiscoverPrivateResponse{},
			serverOut: &ReferrerServiceDiscoverPrivateResponse{CurrentOrgScoped: true},
		},
		{
			name:      "discover response of an older control plane fails closed",
			reply:     &ReferrerServiceDiscoverPrivateResponse{},
			serverOut: &ReferrerServiceDiscoverPrivateResponse{},
			wantErr:   ErrDiscoverScopeNotReported,
		},
		{
			name:      "other methods are not checked",
			reply:     &ReferrerItem{},
			serverOut: &ReferrerItem{},
		},
		{
			name:      "invoke errors are returned as they are",
			reply:     &ReferrerServiceDiscoverPrivateResponse{},
			invokeErr: errInvoke,
			wantErr:   errInvoke,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			invoker := func(_ context.Context, _ string, _, reply any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				if tc.invokeErr != nil {
					return tc.invokeErr
				}
				proto.Merge(reply.(proto.Message), tc.serverOut)
				return nil
			}

			err := DiscoverScopeUnaryClientInterceptor()(context.Background(), "/method", nil, tc.reply, nil, invoker)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}
