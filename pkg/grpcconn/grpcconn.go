//
// Copyright 2024-2026 The Chainloop Authors.
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
	"encoding/base64"
	"fmt"
	"os"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpc_insecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// CLIVersionHeader is the request header key the CLI uses to advertise its
// version (and edition flavor) on every request to the Control Plane and CAS.
// Both gRPC and HTTP treat header keys as case-insensitive, so the same
// canonical name is reused when the Control Plane forwards the value to
// downstream policy providers over HTTP.
const CLIVersionHeader = "Chainloop-Cli-Version"

type newOptionalArg struct {
	caFilePath string
	caContent  string
	insecure   bool
	orgName    string
	cliVersion string
}

type Option func(*newOptionalArg)

func WithCAFile(caFilePath string) Option {
	return func(opt *newOptionalArg) {
		opt.caFilePath = caFilePath
	}
}

// WithCAContent sets the CA certificate content (PEM format or base64-encoded)
func WithCAContent(content string) Option {
	return func(opt *newOptionalArg) {
		opt.caContent = content
	}
}

func WithInsecure(insecure bool) Option {
	return func(opt *newOptionalArg) {
		opt.insecure = insecure
	}
}

func WithOrgName(orgName string) Option {
	return func(opt *newOptionalArg) {
		opt.orgName = orgName
	}
}

// WithCLIVersion attaches the given CLI version (e.g. "v1.94.2-oss") to every
// outgoing request as the chainloop-cli-version header.
func WithCLIVersion(version string) Option {
	return func(opt *newOptionalArg) {
		opt.cliVersion = version
	}
}

func cliVersionUnaryInterceptor(version string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, CLIVersionHeader, version)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func cliVersionStreamInterceptor(version string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, CLIVersionHeader, version)
		return streamer(ctx, desc, cc, method, opts...)
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
	unaryInterceptors := []grpc.UnaryClientInterceptor{}
	streamInterceptors := []grpc.StreamClientInterceptor{}

	if authToken != "" {
		grpcCreds := newTokenAuth(authToken, optionalArgs.insecure, optionalArgs.orgName)
		opts = append(opts, grpc.WithPerRPCCredentials(grpcCreds))
		unaryInterceptors = append(unaryInterceptors, grpc_retry.UnaryClientInterceptor())
	}

	if optionalArgs.cliVersion != "" {
		unaryInterceptors = append(unaryInterceptors, cliVersionUnaryInterceptor(optionalArgs.cliVersion))
		streamInterceptors = append(streamInterceptors, cliVersionStreamInterceptor(optionalArgs.cliVersion))
	}

	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.WithChainStreamInterceptor(streamInterceptors...))
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

		// Load CA from content if provided (takes precedence)
		if optionalArgs.caContent != "" {
			if err = appendCAFromContent(optionalArgs.caContent, certsPool); err != nil {
				return nil, fmt.Errorf("failed to load CA from content: %w", err)
			}
		} else if optionalArgs.caFilePath != "" {
			// Fallback to file path for backward compatibility
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

func appendCAFromContent(content string, certsPool *x509.CertPool) error {
	var pemContent []byte

	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil && len(decoded) > 0 {
		// Successfully decoded as base64
		pemContent = decoded
	} else {
		// Not base64, assume it's PEM content directly
		pemContent = []byte(content)
	}

	// Append to cert pool
	if ok := certsPool.AppendCertsFromPEM(pemContent); !ok {
		return fmt.Errorf("failed to append CA cert to pool")
	}

	return nil
}

type tokenAuth struct {
	token    string
	insecure bool
	orgName  string
}

// Implementation of PerRPCCredentials interface that sends a bearer token in each request.
// https://pkg.go.dev/google.golang.org/grpc/credentials#PerRPCCredentials
func newTokenAuth(token string, insecure bool, orgName string) *tokenAuth {
	return &tokenAuth{token, insecure, orgName}
}

// Return value is mapped to request headers.
func (t tokenAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	const OrganizationHeader = "Chainloop-Organization"
	return map[string]string{
		"authorization":    "Bearer " + t.token,
		OrganizationHeader: t.orgName,
	}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return !t.insecure
}
