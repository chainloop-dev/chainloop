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

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/mocks"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/go-kratos/kratos/v2/log"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func (s *bytestreamSuite) TestStreamReader() {
	buffer := newStreamReader()
	// Write twice and check the length
	err := buffer.Write([]byte("hello"))
	s.NoError(err)
	s.Equal(int64(5), buffer.size)
	err = buffer.Write([]byte("chainloop"))
	s.NoError(err)
	s.Equal(int64(14), buffer.size)
	// The buffer length also matches
	s.Equal(14, buffer.Len())

	// Start reading
	writer := bytes.NewBuffer(nil)
	copied, err := io.Copy(writer, buffer)
	s.Equal(int64(14), copied)
	s.NoError(err)
	// The buffer length is still 14 to indicate what it has processed
	s.Equal(int64(14), buffer.size)
	// but the internal one is 0
	s.Equal(0, buffer.Len())
}

func (s *bytestreamSuite) TestWrite() {
	ctx := s.upCtx

	s.T().Run("Unauthorized", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("unauthenticated", "true"))
		stream, err := s.client.Write(ctx)
		s.NoError(err)
		s.NoError(stream.Send(&bytestream.WriteRequest{}))
		_, err = stream.CloseAndRecv()
		assertGRPCError(t, err, codes.Unauthenticated, "missing authentication")
	})

	s.T().Run("Invalid Role", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(
			context.Background(), metadata.Pairs("role", "downloader"),
		)

		stream, err := s.client.Write(ctx)
		s.NoError(err)
		s.NoError(stream.Send(&bytestream.WriteRequest{}))
		_, err = stream.CloseAndRecv()
		assertGRPCError(t, err, codes.Unauthenticated, "invalid role")
	})

	s.T().Run("missing resource name", func(t *testing.T) {
		stream, err := s.client.Write(ctx)
		s.NoError(err)
		s.NoError(stream.Send(&bytestream.WriteRequest{}))
		_, err = stream.CloseAndRecv()
		assertGRPCError(t, err, codes.InvalidArgument, "resourceName must be set")
	})

	s.T().Run("wrong resource name", func(t *testing.T) {
		stream, err := s.client.Write(ctx)
		s.NoError(err)
		s.NoError(stream.Send(&bytestream.WriteRequest{ResourceName: "wrong"}))
		_, err = stream.CloseAndRecv()
		assertGRPCError(t, err, codes.InvalidArgument, "resourceName must be set")
	})
}

// NOTE: separated test cases for each error case to make sure the context and stubs are re-set
func (s *bytestreamSuite) TestWriteExist() {
	s.ociBackend.On("Exists", mock.Anything, s.resource.Digest).Return(true, nil)

	stream, err := s.client.Write(s.upCtx)
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
	}))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(0), got.CommittedSize)
}

func (s *bytestreamSuite) TestWriteOK() {
	data := []byte("hello world")
	s.ociBackend.On("Exists", mock.Anything, s.resource.Digest).Return(false, nil)
	s.ociBackend.On("Upload", mock.Anything, mock.Anything, s.resource).Return(nil)

	stream, err := s.client.Write(s.upCtx)
	s.NoError(err)
	// Multiple chunks
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         data[:5],
	}))

	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         data[5:],
	}))

	got, err := stream.CloseAndRecv()
	s.NoError(err)
	s.Equal(int64(len(data)), got.CommittedSize)
}

func (s *bytestreamSuite) TestWriteErrorUploading() {
	s.ociBackend.On("Exists", mock.Anything, s.resource.Digest).Return(false, nil)
	s.ociBackend.On("Upload", mock.Anything, mock.Anything, s.resource).Return(errors.New("error uploading"))

	stream, err := s.client.Write(s.upCtx)
	s.NoError(err)
	s.NoError(stream.Send(&bytestream.WriteRequest{
		ResourceName: encodeResource(s.T(), s.resource),
		Data:         []byte("hello world"),
	}))

	_, err = stream.CloseAndRecv()
	s.Error(err)
}

func (s *bytestreamSuite) TestRead() {
	ctx := s.downCtx

	s.T().Run("Unauthorized", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("unauthenticated", "true"))
		reader, err := s.client.Read(ctx, &bytestream.ReadRequest{})
		s.NoError(err)
		_, err = reader.Recv()
		assertGRPCError(t, err, codes.Unauthenticated, "missing authentication")
	})

	s.T().Run("Invalid Role", func(t *testing.T) {
		reader, err := s.client.Read(s.upCtx, &bytestream.ReadRequest{})
		s.NoError(err)
		_, err = reader.Recv()
		assertGRPCError(t, err, codes.Unauthenticated, "invalid role")
	})

	s.T().Run("missing resource name", func(t *testing.T) {
		reader, err := s.client.Read(ctx, &bytestream.ReadRequest{})
		s.NoError(err)
		_, err = reader.Recv()
		assertGRPCError(t, err, codes.InvalidArgument, "empty resource name")
	})
}

func (s *bytestreamSuite) TestReadErrorDownloading() {
	s.ociBackend.On("Download", mock.Anything, mock.Anything, "deadbeef").Return(
		errors.New("error downloading"),
	)

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: "deadbeef"})
	s.NoError(err)

	_, err = reader.Recv()
	assertGRPCError(s.T(), err, codes.Internal, "")
}

func (s *bytestreamSuite) TestDownloadOk() {
	s.ociBackend.On("Download", mock.Anything, mock.Anything, "deadbeef").
		Return(nil).Run(func(args mock.Arguments) {
		buf := bytes.NewBuffer([]byte("hello world"))
		_, err := io.Copy(args.Get(1).(io.Writer), buf)
		s.NoError(err)
	})

	reader, err := s.client.Read(s.downCtx, &bytestream.ReadRequest{ResourceName: "deadbeef"})
	s.NoError(err)

	// receive the data, it should contain all of it since the buffer is serverside is 1MB
	got, err := reader.Recv()
	s.NoError(err)
	s.Equal("hello world", string(got.Data))
	// EOF
	got, err = reader.Recv()
	s.ErrorIs(err, io.EOF)
	s.Nil(got)
}

func assertGRPCError(t *testing.T, err error, code codes.Code, errMsg string) {
	grpcStatus, _ := status.FromError(err)
	assert.Equal(t, code, grpcStatus.Code())
	assert.ErrorContains(t, err, errMsg)
}

func TestBytestreamSuite(t *testing.T) {
	suite.Run(t, new(bytestreamSuite))
}

type bytestreamSuite struct {
	suite.Suite
	conn       *grpc.ClientConn
	srv        *grpc.Server
	client     bytestream.ByteStreamClient
	ociBackend *mocks.UploaderDownloader
	resource   *v1.CASResource
	upCtx      context.Context
	downCtx    context.Context
}

// Run after each test
func (s *bytestreamSuite) TearDownTest() {
	s.conn.Close()
	s.srv.Stop()
}

func (s *bytestreamSuite) SetupTest() {
	const backendType = "backend-type"
	// 1 MB buffer
	l := bufconn.Listen(1 << 20)
	server := grpc.NewServer(
		grpc.StreamInterceptor(
			grpc_auth.StreamServerInterceptor(
				func(ctx context.Context) (context.Context, error) {
					md, _ := metadata.FromIncomingContext(ctx)
					// Simulate unauthenticated
					if v := md.Get("unauthenticated"); len(v) > 0 {
						return ctx, nil
					}

					claims := &casJWT.Claims{
						StoredSecretID: "secret-id", BackendType: backendType,
					}

					if roles := md.Get("role"); len(roles) > 0 {
						if roles[0] == "downloader" {
							claims.Role = casJWT.Downloader
						} else if roles[0] == "uploader" {
							claims.Role = casJWT.Uploader
						}
					}

					return jwtMiddleware.NewContext(ctx, claims), nil
				},
			),
		),
	)
	ociBackendProvider := mocks.NewProvider(s.T())
	ociBackend := mocks.NewUploaderDownloader(s.T())
	ociBackendProvider.On("FromCredentials", mock.Anything, mock.Anything).Maybe().Return(ociBackend, nil)

	bytestream.RegisterByteStreamServer(
		server,
		NewByteStreamService(backend.Providers{
			backendType: ociBackendProvider,
		}, WithLogger(log.DefaultLogger)),
	)
	go func() {
		_ = server.Serve(l)
	}()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return l.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(s.T(), err)

	s.srv = server
	s.conn = conn
	s.ociBackend = ociBackend
	s.client = bytestream.NewByteStreamClient(conn)
	s.resource = &v1.CASResource{
		Digest: "deadbeef", FileName: "skynet.exe",
	}

	s.upCtx = metadata.NewOutgoingContext(context.Background(), metadata.Pairs("role", "uploader"))
	s.downCtx = metadata.NewOutgoingContext(context.Background(), metadata.Pairs("role", "downloader"))
}

// encodedResource returns a base64-encoded v1.UploadResource which wraps both the digest and fileName
func encodeResource(t *testing.T, r *v1.CASResource) string {
	var encodedResource bytes.Buffer
	enc := gob.NewEncoder(&encodedResource)
	err := enc.Encode(r)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(encodedResource.Bytes())
}
