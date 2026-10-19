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
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	reflection "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

// This deploys the chart's CAS Service and Ingress objects in a disposable
// namespace of a caller-supplied local cluster with ingress-nginx installed.
// The CAS runs in this test process; BACKEND_IP must be reachable from its nodes.
// See app/artifact-cas/README.md for the kind setup and invocation.
func TestCASIngressNGINX(t *testing.T) {
	kubeconfig := os.Getenv("CAS_INGRESS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set CAS_INGRESS_KUBECONFIG and CAS_INGRESS_BACKEND_IP for the local ingress-nginx test")
	}
	backendIP := os.Getenv("CAS_INGRESS_BACKEND_IP")
	require.NotNil(t, net.ParseIP(backendIP), "CAS_INGRESS_BACKEND_IP must be reachable from the cluster nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	namespace := fmt.Sprintf("cas-shared-%d", time.Now().UnixNano())
	kubectl := func(input []byte, args ...string) []byte {
		t.Helper()
		full := append([]string{"--kubeconfig", kubeconfig, "--namespace", namespace}, args...)
		cmd := exec.CommandContext(ctx, "kubectl", full...) //nolint:gosec // caller-selected local kubeconfig passed as an argument, without a shell
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		return out
	}
	require.Equal(t, "1", string(kubectl(nil, "get", "configmap", "ingress-nginx-controller", "--namespace", "ingress-nginx", "-o=jsonpath={.data.worker-processes}")), "use one controller worker to exercise both protocols in the same connection cache")
	kubectl(nil, "create", "namespace", namespace)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		out, err := exec.CommandContext(cleanupCtx, "kubectl", "--kubeconfig", kubeconfig, "delete", "namespace", namespace, "--wait=false").CombinedOutput() //nolint:gosec // fixed namespace prefix and caller-selected local kubeconfig, without a shell
		require.NoError(t, err, "%s", out)
	})
	opts := casTestOpts(t)
	opts.ServerConfig.Http.Addr = "0.0.0.0:0"
	opts.ServerConfig.LegacyGrpcListener = new(false)
	sharedURL, _ := startCASTestServers(t, opts)
	_, port, err := net.SplitHostPort(sharedURL[len("http://"):])
	require.NoError(t, err)
	const hostname = "example.com"
	cert := casTestTLS(t)
	kubectl(nil, "create", "secret", "tls", hostname+"-tls", "--cert", cert.Certificate, "--key", cert.PrivateKey)
	helm := exec.CommandContext(ctx, "helm", "template", "probe", "../../../../deployment/chainloop", "--namespace", namespace,
		"--set", "development=true", "--set", "fullnameOverride=cas-probe",
		"--set", "cas.ingress.enabled=true", "--set", "cas.ingress.ingressClassName=nginx",
		"--set", "cas.ingress.hostname="+hostname, "--set", "cas.ingress.tls=true",
		"--show-only", "templates/cas/service-http.yaml", "--show-only", "templates/cas/ingress.yaml")
	helm.Env = append(os.Environ(), "HELM_PLUGINS="+t.TempDir())
	manifest, err := helm.Output()
	require.NoError(t, err)
	kubectl(manifest, "apply", "-f", "-")
	endpointSlice := fmt.Sprintf(`apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: cas-probe-cas-test
  labels:
    kubernetes.io/service-name: cas-probe-cas
    endpointslice.kubernetes.io/managed-by: chainloop-test
addressType: IPv4
ports:
  - name: http
    protocol: TCP
    port: %s
endpoints:
  - addresses: ["%s"]
    conditions:
      ready: true
`, port, backendIP)
	kubectl([]byte(endpointSlice), "apply", "-f", "-")
	lis, err := net.Listen("tcp", casAnyAddr)
	require.NoError(t, err)
	addr := lis.Addr().String()
	localPort := lis.Addr().(*net.TCPAddr).Port
	require.NoError(t, lis.Close())
	forward := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig, "--namespace", "ingress-nginx", "port-forward", "service/ingress-nginx-controller", strconv.Itoa(localPort)+":443") //nolint:gosec // caller-selected local kubeconfig and numeric port, without a shell
	var logs bytes.Buffer
	forward.Stdout, forward.Stderr = &logs, &logs
	require.NoError(t, forward.Start())
	t.Cleanup(func() {
		_ = forward.Process.Kill()
		_ = forward.Wait()
		t.Log(logs.String())
	})
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 10*time.Second, 50*time.Millisecond)
	conn, err := grpcLib.NewClient(addr, grpcLib.WithAuthority(hostname), grpcLib.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))) //nolint:gosec // local test certificate
	require.NoError(t, err)
	defer conn.Close()
	statusClient := v1.NewStatusServiceClient(conn)
	require.Eventually(t, func() bool {
		callCtx, callCancel := context.WithTimeout(ctx, time.Second)
		defer callCancel()
		_, err := statusClient.Statusz(callCtx, &v1.StatuszRequest{})
		return err == nil
	}, 30*time.Second, 500*time.Millisecond, "chart gRPC paths must reach the CAS")
	client := bytestream.NewByteStreamClient(conn)
	content := bytes.Repeat([]byte("CAS ingress streaming\n"), 4096)
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	uploadToken := casTestToken(t, casJWT.Uploader, 0, time.Hour)
	downloadToken := casTestToken(t, casJWT.Downloader, 0, time.Hour)
	require.NoError(t, casTestUpload(t, client, uploadToken, digest, bytes.NewReader(content), int64(len(content))))
	callCtx, callCancel := casTestContext(downloadToken)
	defer callCancel()
	refl, err := reflection.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, refl.Send(&reflection.ServerReflectionRequest{MessageRequest: &reflection.ServerReflectionRequest_ListServices{ListServices: ""}}))
	_, err = refl.Recv()
	require.NoError(t, err, "reflection must use a gRPC upstream too")
	require.NoError(t, refl.CloseSend())
	// Warm both pools repeatedly, in both directions. Reusing a gRPC socket for
	// HTTP (or vice versa) fails after the first successful request on each.
	for _, http2Client := range []bool{false, true} {
		t.Run(fmt.Sprintf("browser_HTTP2_%t", http2Client), func(t *testing.T) {
			tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: http2Client, //nolint:gosec // local test certificate
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, addr)
				}}
			defer tr.CloseIdleConnections()
			browser := &http.Client{Transport: tr, Timeout: 5 * time.Second}
			for range 20 {
				stream, err := client.Read(callCtx, &bytestream.ReadRequest{ResourceName: digest})
				require.NoError(t, err)
				hash := sha256.New()
				for {
					chunk, err := stream.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(t, err)
					_, err = hash.Write(chunk.Data)
					require.NoError(t, err)
				}
				require.Equal(t, digest, hex.EncodeToString(hash.Sum(nil)))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+hostname+"/download/sha256:"+digest, nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+downloadToken)
				resp, err := browser.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Equal(t, map[bool]int{false: 1, true: 2}[http2Client], resp.ProtoMajor)
				hash.Reset()
				_, err = io.Copy(hash, resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				require.Equal(t, digest, hex.EncodeToString(hash.Sum(nil)))
				_, err = statusClient.Statusz(ctx, &v1.StatuszRequest{})
				require.NoError(t, err, "gRPC after the HTTP pool is warm")
				resp, err = browser.Get("https://" + hostname + "/statusz")
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
			}
		})
	}
}
