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

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/stretchr/testify/require"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Run on Linux with Docker and an available image:
// NGINX_INTEGRATION_IMAGE=nginx:1.28.0-alpine go test ./app/controlplane/internal/server -run TestNGINXSharedIngress -v
// This checks the grpc_pass path used by the chart, not a Kubernetes controller.
func TestNGINXSharedIngress(t *testing.T) {
	image := os.Getenv("NGINX_INTEGRATION_IMAGE")
	if image == "" {
		t.Skip("set NGINX_INTEGRATION_IMAGE to run the Docker integration test")
	}
	off := false
	upstream, _, _ := startSinglePortApp(t, &off)
	cert, key := writeSelfSignedCert(t)
	dir := t.TempDir()
	for name, src := range map[string]string{"cert.pem": cert, "key.pem": key} {
		data, err := os.ReadFile(src)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600)) //nolint:gosec // fixed filenames inside the test's temporary directory
	}
	lis, err := net.Listen("tcp", testAnyAddr)
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	config := fmt.Sprintf(`events { worker_connections 128; }
http {
    upstream controlplane { server %s; keepalive 16; }
    server {
        listen %s ssl http2;
        ssl_certificate /probe/cert.pem;
        ssl_certificate_key /probe/key.pem;
        location / { grpc_pass grpc://controlplane; }
    }
}
`, upstream, addr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nginx.conf"), []byte(config), 0o600))
	name := fmt.Sprintf("chainloop-shared-ingress-%d-%s", os.Getpid(), strings.ReplaceAll(addr, ":", "-"))
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, //nolint:gosec // test-selected image passed as an argument, without a shell
		"--network", "host", "-v", dir+":/probe:ro", image,
		"nginx", "-c", "/probe/nginx.conf", "-g", "daemon off;")
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = exec.CommandContext(stopCtx, "docker", "stop", "--timeout", "1", name).Run()
		cancel()
		_ = cmd.Wait()
		t.Log(logs.String())
	})
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 10*time.Second, 50*time.Millisecond)

	conn, err := grpcLib.NewClient(addr, grpcLib.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{InsecureSkipVerify: true}))) //nolint:gosec // test certificate
	require.NoError(t, err)
	defer conn.Close()
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rpcCancel()
	_, err = v1.NewStatusServiceClient(conn).Statusz(rpcCtx, &v1.StatuszRequest{})
	require.NoError(t, err, "native gRPC must succeed before browser requests")

	for _, useHTTP2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("client_HTTP2_%t", useHTTP2), func(t *testing.T) {
			tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: useHTTP2} //nolint:gosec // test certificate
			defer tr.CloseIdleConnections()
			client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
			for range 3 {
				resp, err := client.Get("https://" + addr + "/statusz")
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Equal(t, map[bool]int{false: 1, true: 2}[useHTTP2], resp.ProtoMajor)

				url := "https://" + addr + "/controlplane.v1.StatusService/Statusz"
				req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte{0, 0, 0, 0, 0}))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/grpc-web+proto")
				req.Header.Set("X-Grpc-Web", "1")
				resp, err = client.Do(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Contains(t, resp.Header.Get("Content-Type"), "application/grpc-web")
				require.Contains(t, string(body), "grpc-status: 0")

				req, err = http.NewRequest(http.MethodOptions, url, nil)
				require.NoError(t, err)
				req.Header.Set("Origin", "https://example.test")
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "content-type,x-grpc-web")
				resp, err = client.Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusNoContent, resp.StatusCode)
				require.Equal(t, "https://example.test", resp.Header.Get("Access-Control-Allow-Origin"))
			}
		})
	}
}
