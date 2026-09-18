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

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"

	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	s3backend "github.com/chainloop-dev/chainloop/pkg/blobmanager/s3"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestUploadEndToEndMinio drives the whole upload path against real
// infrastructure: a gRPC client talking over a TCP socket to the real
// ByteStreamService, which stages the artifact on a real filesystem and hands
// the resulting file to the real S3 backend writing into a real MinIO server.
// Nothing here is mocked except the credential provider, which only decides
// which backend to hand back.
//
// This is what the mock-backed suite cannot show: that the staged *os.File is
// something the AWS SDK actually accepts and uploads correctly, that the bytes
// landing in the object store are the bytes the client sent, and that a
// rejected upload leaves the bucket untouched.
func TestUploadEndToEndMinio(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip()
	}

	const bucket = "e2e-bucket"
	endpoint := startMinio(t)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds: miniocreds.NewStaticV4("root", "test-password", ""), Secure: false,
	})
	require.NoError(t, err)
	require.NoError(t, minioClient.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}))

	realBackend, err := s3backend.NewBackend(&s3backend.Credentials{
		AccessKeyID:     "root",
		SecretAccessKey: "test-password",
		Region:          "us-east-1",
		Location:        fmt.Sprintf("http://%s/%s", endpoint, bucket),
	})
	require.NoError(t, err)

	stagingDir := t.TempDir()
	client, peakStagedBytes := newE2EClient(t, realBackend, stagingDir)

	t.Run("multipart upload lands byte-identical in the bucket", func(t *testing.T) {
		// 12 MB forces the AWS SDK past its 5 MB part size, so the staged file
		// is read back in parts rather than in one shot.
		content := deterministicBytes(12 << 20)
		digest := sha256Hex(content)
		resource := &v1.CASResource{Digest: digest, FileName: "big-artifact.bin"}

		stream, err := client.Write(e2eUploadCtx())
		require.NoError(t, err)
		sendInChunks(t, stream, encodeResource(t, resource), content, 64<<10)

		resp, err := stream.CloseAndRecv()
		require.NoError(t, err)
		require.Equal(t, int64(len(content)), resp.GetCommittedSize())

		// The object really is in MinIO, and it really is the bytes we sent.
		obj, err := minioClient.GetObject(context.Background(), bucket, "sha256:"+digest, minio.GetObjectOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = obj.Close() })

		stored := sha256.New()
		written, err := copyInto(stored, obj)
		require.NoError(t, err)
		require.Equal(t, int64(len(content)), written, "stored object size must match what was uploaded")
		require.Equal(t, digest, hex.EncodeToString(stored.Sum(nil)),
			"the object stored under the canonical key must hash to that digest")

		require.Empty(t, listStaging(t, stagingDir), "staging dir must never hold an entry")
		require.Positive(t, peakStagedBytes.Load(),
			"the upload must actually spill to disk rather than buffer in memory")
		require.Empty(t, openStagingFDs(t), "the staging descriptor must be released")
	})

	t.Run("digest mismatch is rejected and nothing is written to the bucket", func(t *testing.T) {
		content := []byte("bytes that do not hash to the declared digest")
		declared := sha256Hex([]byte("a completely different artifact"))
		resource := &v1.CASResource{Digest: declared, FileName: "tampered.bin"}

		stream, err := client.Write(e2eUploadCtx())
		require.NoError(t, err)
		sendInChunks(t, stream, encodeResource(t, resource), content, 8<<10)

		_, err = stream.CloseAndRecv()
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Contains(t, err.Error(), "does not match the declared digest")

		// The canonical key must not exist: unverified bytes never reached S3.
		_, err = minioClient.StatObject(context.Background(), bucket, "sha256:"+declared, minio.StatObjectOptions{})
		require.Error(t, err, "no object may be written under a digest that was never verified")
		require.Equal(t, "NoSuchKey", minio.ToErrorResponse(err).Code)

		require.Empty(t, listStaging(t, stagingDir), "staging dir must never hold an entry")
		require.Empty(t, openStagingFDs(t), "the staging descriptor must be released after a rejection")
	})
}

// newE2EClient wires the real ByteStreamService to realBackend over a TCP gRPC
// connection. The returned counter records the largest staged artifact seen on
// disk while the server was running, so a test can prove the upload spilled
// rather than buffered. Staging files are unlinked at creation, so the size is
// read through the open descriptor rather than from the directory.
func newE2EClient(t *testing.T, realBackend backend.UploaderDownloader, stagingDir string) (bytestream.ByteStreamClient, *atomic.Int64) {
	t.Helper()

	const backendType = "s3-e2e"

	provider := mocks.NewProvider(t)
	provider.On("FromCredentials", mock.Anything, mock.Anything).Maybe().Return(realBackend, nil)

	server := grpc.NewServer(
		grpc.StreamInterceptor(
			grpc_auth.StreamServerInterceptor(func(ctx context.Context) (context.Context, error) {
				return jwtMiddleware.NewContext(ctx, &casJWT.Claims{
					StoredSecretID: testStoredSecretID,
					BackendType:    backendType,
					OrgID:          testOrgID,
					Role:           casJWT.Uploader,
				}), nil
			}),
		),
	)

	bytestream.RegisterByteStreamServer(server, NewByteStreamService(
		backend.Providers{backendType: provider},
		WithLogger(log.DefaultLogger),
		WithAuditDispatcher(newTestDispatcher(&fakePublisher{})),
		WithStagingDir(stagingDir),
	))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Sample the staged artifact while the server runs so the test can assert it
	// was really written to disk mid-upload.
	var peak atomic.Int64
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				if n := stagedBytes(); n > peak.Load() {
					peak.Store(n)
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

	return bytestream.NewByteStreamClient(conn), &peak
}

// listStaging returns the CAS staging files visible in the directory. Staging
// files are unlinked at creation, so this must stay empty at every moment of an
// upload, not only after one.
func listStaging(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stagingFilePrefix) {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	return found
}

// stagedBytes returns the size of the largest staging file this process holds
// open. The files are unlinked, so their size is only reachable through the
// descriptor; a non-zero reading during an upload is what proves the artifact
// went to disk instead of staying in memory.
func stagedBytes() int64 {
	if runtime.GOOS != "linux" {
		return 0
	}

	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0
	}

	var largest int64
	for _, e := range entries {
		fdPath := filepath.Join("/proc/self/fd", e.Name())
		target, err := os.Readlink(fdPath)
		if err != nil || !strings.Contains(target, stagingFilePrefix) {
			continue
		}
		// Stat through /proc/self/fd/N, which follows the descriptor rather than
		// the (now absent) path.
		info, err := os.Stat(fdPath)
		if err != nil {
			continue
		}
		if info.Size() > largest {
			largest = info.Size()
		}
	}
	return largest
}

func e2eUploadCtx() context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("role", "uploader"))
}

func startMinio(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	const port = "9000/tcp"
	instance, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			// Same pinned image the s3 backend suite uses.
			Image:        "quay.io/minio/minio@sha256:a1a8bd4ac40ad7881a245bab97323e18f971e4d4cba2c2007ec1bedd21cbaba2",
			ExposedPorts: []string{port},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "root",
				"MINIO_ROOT_PASSWORD": "test-password",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForListeningPort(port).WithStartupTimeout(5 * time.Minute),
		},
		Started: true,
	})
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, instance, testcontainers.StopTimeout(time.Minute))

	p, err := instance.MappedPort(ctx, "9000")
	require.NoError(t, err)

	return fmt.Sprintf("127.0.0.1:%d", p.Num())
}

// copyInto streams r into w, returning the number of bytes copied.
func copyInto(w io.Writer, r io.Reader) (int64, error) {
	return io.Copy(w, r)
}
