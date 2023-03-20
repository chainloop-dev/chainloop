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

package grpcconn

import (
	"context"
	"crypto/x509"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpc_insecure "google.golang.org/grpc/credentials/insecure"
)

func New(uri, authToken string, insecure bool) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if authToken != "" {
		grpcCreds := newTokenAuth(authToken, insecure)

		opts = []grpc.DialOption{
			grpc.WithPerRPCCredentials(grpcCreds),
			// Retry using default configuration
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
		}
	}

	var tlsDialOption grpc.DialOption
	if insecure {
		tlsDialOption = grpc.WithTransportCredentials(grpc_insecure.NewCredentials())
	} else {
		certsPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		tlsDialOption = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certsPool, ""))
	}

	opts = append(opts, tlsDialOption)

	conn, err := grpc.Dial(uri, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

type tokenAuth struct {
	token    string
	insecure bool
}

// Implementation of PerRPCCredentials interface that sends a bearer token in each request.
func newTokenAuth(token string, insecure bool) *tokenAuth {
	return &tokenAuth{token, insecure}
}

// Return value is mapped to request headers.
func (t tokenAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return !t.insecure
}
