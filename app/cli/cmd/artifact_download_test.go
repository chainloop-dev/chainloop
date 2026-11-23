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

package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	casv1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	testCASToken    = "cas-header.cas-claims.cas-signature"
	testMintedToken = "minted-header.minted-claims.minted-signature"
	testAPIToken    = "api-token"
)

func TestArtifactDownloadCASCredential(t *testing.T) {
	content := []byte("hello world")
	sum := sha256.Sum256(content)
	digestHex := hex.EncodeToString(sum[:])

	testCases := []struct {
		name     string
		env      map[string]string
		args     []string
		casToken string
		// wantCPToken is the API token of the CAS credentials request, empty when the control plane gets no call
		wantCPToken  string
		wantCASToken string
		wantErr      string
	}{
		{
			name:         "the CAS credential from the environment downloads without the control plane",
			env:          map[string]string{artifactCASTokenEnvVarName: testCASToken},
			casToken:     testCASToken,
			wantCASToken: testCASToken,
		},
		{
			name:         "the CAS credential from the flag downloads without the control plane",
			args:         []string{"--artifact-cas-token", testCASToken},
			casToken:     testCASToken,
			wantCASToken: testCASToken,
		},
		{
			name:         "the CAS credential has precedence over the API token",
			env:          map[string]string{artifactCASTokenEnvVarName: testCASToken, tokenEnvVarName: testAPIToken},
			casToken:     testCASToken,
			wantCASToken: testCASToken,
		},
		{
			name:         "without a CAS credential the control plane gives one for the digest",
			env:          map[string]string{tokenEnvVarName: testAPIToken},
			casToken:     testMintedToken,
			wantCPToken:  testAPIToken,
			wantCASToken: testMintedToken,
		},
		{
			name:     "a value that is not a JWT fails before any request",
			env:      map[string]string{artifactCASTokenEnvVarName: "not-a-jwt"},
			casToken: testCASToken,
			wantErr:  "the CAS credential is not a JWT",
		},
		{
			name:         "a credential that the CAS does not accept is reported",
			env:          map[string]string{artifactCASTokenEnvVarName: testCASToken},
			casToken:     "other-header.other-claims.other-signature",
			wantCASToken: testCASToken,
			wantErr:      "the CAS did not accept the credential",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cp := &fakeCASCredentials{token: testMintedToken}
			cpAddr, cpCalls := startFakeGRPCServer(t, func(s *grpc.Server) {
				pb.RegisterCASCredentialsServiceServer(s, cp)
			})

			cas := &fakeCAS{token: tc.casToken, digestHex: digestHex, content: content}
			casAddr, _ := startFakeGRPCServer(t, func(s *grpc.Server) {
				casv1.RegisterResourceServiceServer(s, cas)
				bytestream.RegisterByteStreamServer(s, cas)
			})

			// The environment of the test run must not provide credentials
			t.Setenv(tokenEnvVarName, "")
			t.Setenv(artifactCASTokenEnvVarName, "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			outputFile := filepath.Join(t.TempDir(), "artifact.txt")
			args := append([]string{"artifact", "download", "--digest", "sha256:" + digestHex, "--output", outputFile}, tc.args...)
			err := runTestCommand(t, cpAddr, casAddr, args...)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				got, err := os.ReadFile(outputFile)
				require.NoError(t, err)
				assert.Equal(t, content, got)
			}

			assert.Equal(t, tc.wantCASToken, cas.lastToken())

			if tc.wantCPToken == "" {
				assert.Empty(t, cpCalls(), "the control plane must not get a call")
				return
			}

			assert.Equal(t, 1, countOf(cpCalls(), pb.CASCredentialsService_Get_FullMethodName))
			req, token := cp.lastRequest()
			assert.Equal(t, tc.wantCPToken, token)
			assert.True(t, proto.Equal(&pb.CASCredentialsServiceGetRequest{
				Role:          pb.CASCredentialsServiceGetRequest_ROLE_DOWNLOADER,
				Digest:        "sha256:" + digestHex,
				ScopeToDigest: true,
			}, req), "unexpected request: %v", req)
		})
	}
}

// The command builds its control plane connection itself, and telemetry still reports it
func TestArtifactDownloadIsReported(t *testing.T) {
	assert.True(t, shouldReportCommand(newArtifactDownloadCmd()))
}

// runTestCommand runs the CLI with a temporary config file against the given servers.
// It sets no organization, so the root initialization would ask the control plane for one.
func runTestCommand(t *testing.T, cpAddr, casAddr string, args ...string) error {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
	prevActionOpts, prevLogger := ActionOpts, logger
	ActionOpts = nil
	t.Cleanup(func() { ActionOpts, logger = prevActionOpts, prevLogger })

	configFile := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configFile, nil, 0o600))

	root := NewRootCmd(zerolog.Nop())
	root.SetArgs(append(args,
		"--config", configFile,
		"--control-plane", cpAddr,
		"--artifact-cas", casAddr,
		"-i", // the test servers have no TLS
	))

	return root.Execute()
}

// startFakeGRPCServer serves the registered services on a local port and records the methods it gets
func startFakeGRPCServer(t *testing.T, register func(*grpc.Server)) (string, func() []string) {
	t.Helper()

	var mu sync.Mutex
	var calls []string
	record := func(method string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, method)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			record(info.FullMethod)
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			record(info.FullMethod)
			return handler(srv, ss)
		}),
		// The interceptors do not see the methods of services that are not registered
		grpc.UnknownServiceHandler(func(_ any, ss grpc.ServerStream) error {
			method, _ := grpc.MethodFromServerStream(ss)
			record(method)
			return status.Error(codes.Unimplemented, "unknown service")
		}),
	)
	register(s)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)

	return l.Addr().String(), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

func countOf(calls []string, method string) int {
	n := 0
	for _, c := range calls {
		if c == method {
			n++
		}
	}

	return n
}

func bearerToken(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get("authorization"); len(v) > 0 && len(v[0]) > len("Bearer ") {
		return v[0][len("Bearer "):]
	}

	return ""
}

// fakeCASCredentials returns a fixed CAS credential and records the last request
type fakeCASCredentials struct {
	pb.UnimplementedCASCredentialsServiceServer
	token string

	mu      sync.Mutex
	req     *pb.CASCredentialsServiceGetRequest
	reqAuth string
}

func (f *fakeCASCredentials) Get(ctx context.Context, req *pb.CASCredentialsServiceGetRequest) (*pb.CASCredentialsServiceGetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.req, f.reqAuth = req, bearerToken(ctx)

	return &pb.CASCredentialsServiceGetResponse{Result: &pb.CASCredentialsServiceGetResponse_Result{Token: f.token}}, nil
}

func (f *fakeCASCredentials) lastRequest() (*pb.CASCredentialsServiceGetRequest, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.req, f.reqAuth
}

// fakeCAS serves one artifact to requests with the expected credential
type fakeCAS struct {
	casv1.UnimplementedResourceServiceServer
	bytestream.UnimplementedByteStreamServer
	token     string
	digestHex string
	content   []byte

	mu   sync.Mutex
	seen string
}

func (f *fakeCAS) authorize(ctx context.Context) error {
	got := bearerToken(ctx)

	f.mu.Lock()
	f.seen = got
	f.mu.Unlock()

	if got != f.token {
		return status.Error(codes.Unauthenticated, "unexpected credential")
	}

	return nil
}

func (f *fakeCAS) lastToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen
}

func (f *fakeCAS) Describe(ctx context.Context, req *casv1.ResourceServiceDescribeRequest) (*casv1.ResourceServiceDescribeResponse, error) {
	if err := f.authorize(ctx); err != nil {
		return nil, err
	}

	if req.GetDigest() != f.digestHex {
		return nil, status.Error(codes.NotFound, "artifact not found")
	}

	return &casv1.ResourceServiceDescribeResponse{Result: &casv1.CASResource{
		FileName: "artifact.txt", Digest: f.digestHex, Size: int64(len(f.content)),
	}}, nil
}

func (f *fakeCAS) Read(req *bytestream.ReadRequest, stream bytestream.ByteStream_ReadServer) error {
	if err := f.authorize(stream.Context()); err != nil {
		return err
	}

	if req.GetResourceName() != f.digestHex {
		return status.Error(codes.NotFound, "artifact not found")
	}

	return stream.Send(&bytestream.ReadResponse{Data: f.content})
}
