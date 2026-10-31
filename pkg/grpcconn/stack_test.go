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

package grpcconn

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestStackUnaryClientInterceptor(t *testing.T) {
	statusErr := status.Error(codes.NotFound, "missing")

	testCases := []struct {
		name      string
		invokeErr error
		wantStack bool
	}{
		{name: "nil stays nil", invokeErr: nil},
		{name: "status error gets a stack", invokeErr: statusErr, wantStack: true},
		{name: "plain error gets a stack", invokeErr: errors.New("boom"), wantStack: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
				return tc.invokeErr
			}

			err := StackUnaryClientInterceptor()(context.Background(), "/svc/Method", nil, nil, nil, invoker)
			if tc.invokeErr == nil {
				assert.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tc.invokeErr)
			assert.Equal(t, tc.invokeErr.Error(), err.Error())
			assert.Equal(t, tc.wantStack, errorsx.HasStack(err))
			assert.Equal(t, status.Code(tc.invokeErr), status.Code(err))
		})
	}
}

// fakeClientStream returns the configured errors from its stream methods.
type fakeClientStream struct {
	grpc.ClientStream
	sendErr, recvErr, closeErr, headerErr error
}

func (s *fakeClientStream) SendMsg(any) error            { return s.sendErr }
func (s *fakeClientStream) RecvMsg(any) error            { return s.recvErr }
func (s *fakeClientStream) CloseSend() error             { return s.closeErr }
func (s *fakeClientStream) Header() (metadata.MD, error) { return nil, s.headerErr }

func TestStackStreamClientInterceptor(t *testing.T) {
	statusErr := status.Error(codes.Unavailable, "down")

	t.Run("stream creation error gets a stack", func(t *testing.T) {
		streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
			return nil, statusErr
		}

		cs, err := StackStreamClientInterceptor()(context.Background(), &grpc.StreamDesc{}, nil, "/svc/Stream", streamer)
		assert.Nil(t, cs)
		require.ErrorIs(t, err, statusErr)
		assert.True(t, errorsx.HasStack(err))
		assert.Equal(t, codes.Unavailable, status.Code(err))
	})

	testCases := []struct {
		name      string
		stream    *fakeClientStream
		call      func(grpc.ClientStream) error
		wantErr   error
		wantStack bool
	}{
		{
			name:      "SendMsg error gets a stack",
			stream:    &fakeClientStream{sendErr: statusErr},
			call:      func(cs grpc.ClientStream) error { return cs.SendMsg(nil) },
			wantErr:   statusErr,
			wantStack: true,
		},
		{
			name:      "RecvMsg error gets a stack",
			stream:    &fakeClientStream{recvErr: statusErr},
			call:      func(cs grpc.ClientStream) error { return cs.RecvMsg(nil) },
			wantErr:   statusErr,
			wantStack: true,
		},
		{
			name:      "CloseSend error gets a stack",
			stream:    &fakeClientStream{closeErr: statusErr},
			call:      func(cs grpc.ClientStream) error { return cs.CloseSend() },
			wantErr:   statusErr,
			wantStack: true,
		},
		{
			name:   "Header error gets a stack",
			stream: &fakeClientStream{headerErr: statusErr},
			call: func(cs grpc.ClientStream) error {
				_, err := cs.Header()
				return err
			},
			wantErr:   statusErr,
			wantStack: true,
		},
		{
			name:    "RecvMsg io.EOF stays io.EOF",
			stream:  &fakeClientStream{recvErr: io.EOF},
			call:    func(cs grpc.ClientStream) error { return cs.RecvMsg(nil) },
			wantErr: io.EOF,
		},
		{
			name:    "SendMsg io.EOF stays io.EOF",
			stream:  &fakeClientStream{sendErr: io.EOF},
			call:    func(cs grpc.ClientStream) error { return cs.SendMsg(nil) },
			wantErr: io.EOF,
		},
		{
			name:   "nil stays nil",
			stream: &fakeClientStream{},
			call:   func(cs grpc.ClientStream) error { return cs.RecvMsg(nil) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
				return tc.stream, nil
			}

			cs, err := StackStreamClientInterceptor()(context.Background(), &grpc.StreamDesc{}, nil, "/svc/Stream", streamer)
			require.NoError(t, err)

			err = tc.call(cs)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}

			if !tc.wantStack {
				// Callers compare io.EOF with ==, so it must not be wrapped.
				assert.Same(t, tc.wantErr, err) //nolint:testifylint
				return
			}

			require.ErrorIs(t, err, tc.wantErr)
			assert.True(t, errorsx.HasStack(err))
			assert.Equal(t, status.Code(tc.wantErr), status.Code(err))
		})
	}
}
