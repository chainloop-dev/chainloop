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

package action

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestArtifactUploadRun(t *testing.T) {
	content := []byte("hello world")
	sum := sha256.Sum256(content)
	digestHex := hex.EncodeToString(sum[:])

	testCases := []struct {
		name string
		// replace swaps the file at the path after it was opened and hashed
		replace bool
	}{
		{name: "uploads the file with its digest"},
		{name: "uploads the hashed content when the file is replaced", replace: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "artifact.txt")
			require.NoError(t, os.WriteFile(filePath, content, 0o600))

			f, err := os.Open(filePath)
			require.NoError(t, err)
			defer f.Close()

			if tc.replace {
				other := filepath.Join(t.TempDir(), "other.txt")
				require.NoError(t, os.WriteFile(other, []byte("other content"), 0o600))
				require.NoError(t, os.Rename(other, filePath))
			}

			srv := &fakeByteStream{}
			upload := NewArtifactUpload(&ArtifactUploadOpts{
				ActionsOpts:      &ActionsOpts{Logger: zerolog.Nop()},
				ArtifactsCASConn: newFakeByteStreamConn(t, srv),
			})

			got, err := upload.Run(f, "sha256:"+digestHex)
			require.NoError(t, err)
			assert.Equal(t, "sha256:"+digestHex, got.Digest)

			assert.Equal(t, digestHex, srv.resource.Digest)
			assert.Equal(t, "artifact.txt", srv.resource.FileName)
			assert.Equal(t, content, srv.data.Bytes())
		})
	}
}

// fakeByteStream records the resource and the bytes of a single upload
type fakeByteStream struct {
	bytestream.UnimplementedByteStreamServer
	resource v1.CASResource
	data     bytes.Buffer
}

func (s *fakeByteStream) Write(stream bytestream.ByteStream_WriteServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&bytestream.WriteResponse{CommittedSize: int64(s.data.Len())})
		} else if err != nil {
			return err
		}

		if s.resource.Digest == "" {
			raw, err := base64.StdEncoding.DecodeString(req.GetResourceName())
			if err != nil {
				return err
			}
			if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&s.resource); err != nil {
				return err
			}
		}

		s.data.Write(req.GetData())
	}
}

func newFakeByteStreamConn(t *testing.T, srv bytestream.ByteStreamServer) *grpc.ClientConn {
	t.Helper()

	l := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	bytestream.RegisterByteStreamServer(s, srv)
	go func() { _ = s.Serve(l) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return l.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		s.Stop()
	})

	return conn
}
