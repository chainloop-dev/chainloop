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
	"fmt"
	"os"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpc_insecure "google.golang.org/grpc/credentials/insecure"
)

type newOptionalArg struct {
	caFilePath string
	insecure   bool
}

type Option func(*newOptionalArg)

func WithCAFile(caFilePath string) Option {
	return func(opt *newOptionalArg) {
		opt.caFilePath = caFilePath
	}
}

func WithInsecure(insecure bool) Option {
	return func(opt *newOptionalArg) {
		opt.insecure = insecure
	}
}

// Simple wrapper around grpc.Dial that returns a grpc.ClientConn
// It sets up the connection with the correct credentials headers
func New(uri, authToken string, opt ...Option) (*grpc.ClientConn, error) {
	optionalArgs := &newOptionalArg{}
	for _, o := range opt {
		o(optionalArgs)
	}

	var opts []grpc.DialOption
	if authToken != "" {
		grpcCreds := newTokenAuth(authToken, optionalArgs.insecure)

		opts = []grpc.DialOption{
			grpc.WithPerRPCCredentials(grpcCreds),
			// Retry using default configuration
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
		}
	}

	// Currently we only support system tls certs
	var tlsDialOption grpc.DialOption
	if optionalArgs.insecure {
		tlsDialOption = grpc.WithTransportCredentials(grpc_insecure.NewCredentials())
	} else {
		var err error
		certsPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		if optionalArgs.caFilePath != "" {
			if err = appendCAFromFile(optionalArgs.caFilePath, certsPool); err != nil {
				return nil, fmt.Errorf("failed to load CA cert: %w", err)
			}
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

func appendCAFromFile(path string, certsPool *x509.CertPool) error {
	// Load CA cert
	caCert, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}

	if ok := certsPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to append CA cert to pool")
	}

	return nil
}

type tokenAuth struct {
	token    string
	insecure bool
}

// Implementation of PerRPCCredentials interface that sends a bearer token in each request.
// https://pkg.go.dev/google.golang.org/grpc/credentials#PerRPCCredentials
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
