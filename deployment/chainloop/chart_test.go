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

package chart_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The control plane serves every protocol per request on its shared ingress
// (replacement spec D-001/D-002), so the whole ingress uses the gRPC backend
// protocol and the Traefik h2c scheme. The CAS classifies connections with
// cmux and its native transport (D-017), so its shared HTTP ingress keeps
// HTTP/1.1 and the gRPC RPC paths are split into a second Ingress with their
// own upstream pool (D-018). The legacy gRPC resources keep their own upstreams.
func TestSharedIngress(t *testing.T) {
	t.Run("controlplane", func(t *testing.T) { testSharedControlplaneIngress(t) })
	t.Run("cas", func(t *testing.T) { testSharedCASIngress(t) })
}

func renderChart(t *testing.T, args []string) map[string]chartResource {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is required for chart rendering tests")
	}
	full := append([]string{"template", "test", ".", "--set", "development=true",
		"--set", "controlplane.ingressAPI.enabled=true", "--set", "cas.ingressAPI.enabled=true"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helm, full...)
	// Local plugins are unrelated to chart rendering and can print
	// startup warnings on stdout before the YAML document.
	cmd.Env = append(os.Environ(), "HELM_PLUGINS="+t.TempDir())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err, "%s", stderr.String())
	docs := yaml.NewDecoder(bytes.NewReader(out))
	resources := make(map[string]chartResource)
	for {
		var resource chartResource
		err := docs.Decode(&resource)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		resources[resource.Kind+"/"+resource.Metadata.Name] = resource
	}
	return resources
}

// The control plane dispatches each request on one shared HTTP ingress, with
// the gRPC upstream protocol applied to the whole ingress.
func testSharedControlplaneIngress(t *testing.T) {
	const (
		grpcBackend   = "GRPC"
		h2cScheme     = "h2c"
		chartFullname = "test-chainloop"
	)
	for _, tc := range []struct {
		name            string
		args            []string
		fullname        string
		backendProtocol string
		traefikScheme   string
	}{
		{name: "defaults", fullname: chartFullname, backendProtocol: grpcBackend, traefikScheme: h2cScheme},
		{name: "custom name", args: []string{"--set", "fullnameOverride=shared-api"}, fullname: "shared-api", backendProtocol: grpcBackend, traefikScheme: h2cScheme},
		{
			name:            "common annotations merge",
			args:            []string{"--set-string", `commonAnnotations.nginx\.org/http2=false`},
			fullname:        chartFullname,
			backendProtocol: grpcBackend, traefikScheme: h2cScheme,
		},
		{
			name: "encrypted upstream overrides",
			args: []string{
				"--set-string", `controlplane.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				"--set-string", `controlplane.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname: chartFullname, backendProtocol: "GRPCS", traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-controlplane"
			resources := renderChart(t, append(tc.args,
				"--set", "controlplane.ingress.enabled=true", "--set", "controlplane.ingress.hostname=shared.example.test"))
			ingress, ok := resources["Ingress/"+serviceName]
			require.True(t, ok)
			require.Equal(t, tc.backendProtocol, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Equal(t, serviceName, ingress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Equal(t, "true", ingress.Metadata.Annotations["nginx.org/http2"])
			require.Len(t, ingress.Spec.Rules, 1)
			require.Equal(t, "shared.example.test", ingress.Spec.Rules[0].Host)
			require.Len(t, ingress.Spec.Rules[0].HTTP.Paths, 1)
			path := ingress.Spec.Rules[0].HTTP.Paths[0]
			require.Equal(t, "/", path.Path)
			require.Equal(t, serviceName, path.Backend.Service.Name)
			require.Equal(t, "http", path.Backend.Service.Port.Name)
			svc := resources["Service/"+serviceName]
			require.Equal(t, tc.traefikScheme, svc.Metadata.Annotations["traefik.ingress.kubernetes.io/service.serversscheme"])
			legacy := resources["Ingress/"+serviceName+"-api"]
			require.Equal(t, grpcBackend, legacy.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Len(t, legacy.Spec.Rules, 1)
			require.Equal(t, serviceName+"-api", legacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		})
	}
}

// The CAS shared HTTP ingress keeps an HTTP/1.1 upstream, and the gRPC RPC
// paths render into a second Ingress on the same hostname with their own
// upstream pool (spec D-018).
func testSharedCASIngress(t *testing.T) {
	const (
		grpcBackend   = "GRPC"
		chartFullname = "test-chainloop"
	)
	for _, tc := range []struct {
		name          string
		args          []string
		fullname      string
		grpcProtocol  string
		sharedHTTP2   string
		grpcHTTP2     string
		traefikScheme string
	}{
		{name: "defaults", fullname: chartFullname, grpcProtocol: grpcBackend, grpcHTTP2: "true"},
		{name: "custom name", args: []string{"--set", "fullnameOverride=shared-api"}, fullname: "shared-api", grpcProtocol: grpcBackend, grpcHTTP2: "true"},
		{
			// A global annotation reaches both Ingress objects; on the gRPC
			// Ingress it overrides the chart default.
			name:         "common annotations merge",
			args:         []string{"--set-string", `commonAnnotations.nginx\.org/http2=false`},
			fullname:     chartFullname,
			grpcProtocol: grpcBackend, sharedHTTP2: "false", grpcHTTP2: "false",
		},
		{
			name: "encrypted upstream overrides",
			args: []string{
				"--set-string", `cas.ingress.grpc.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				"--set-string", `cas.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname: chartFullname,
			grpcProtocol: "GRPCS", sharedHTTP2: "", grpcHTTP2: "true", traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-cas"
			resources := renderChart(t, append(tc.args,
				"--set", "cas.ingress.enabled=true", "--set", "cas.ingress.hostname=shared.example.test"))

			// The shared HTTP ingress dials HTTP/1.1: no gRPC annotations by default.
			ingress, ok := resources["Ingress/"+serviceName]
			require.True(t, ok)
			require.Empty(t, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Equal(t, tc.sharedHTTP2, ingress.Metadata.Annotations["nginx.org/http2"])
			require.Empty(t, ingress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Len(t, ingress.Spec.Rules, 1)
			require.Equal(t, "shared.example.test", ingress.Spec.Rules[0].Host)
			require.Len(t, ingress.Spec.Rules[0].HTTP.Paths, 1)
			path := ingress.Spec.Rules[0].HTTP.Paths[0]
			require.Equal(t, "/", path.Path)
			require.Equal(t, serviceName, path.Backend.Service.Name)
			require.Equal(t, "http", path.Backend.Service.Port.Name)

			// The gRPC RPC paths render into their own Ingress on the same
			// hostname, with their own annotations and upstream pool.
			grpcIngress, ok := resources["Ingress/"+serviceName+"-grpc"]
			require.True(t, ok)
			require.Equal(t, tc.grpcProtocol, grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Equal(t, serviceName, grpcIngress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Equal(t, tc.grpcHTTP2, grpcIngress.Metadata.Annotations["nginx.org/http2"])
			require.Equal(t, "100m", grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"])
			require.Equal(t, "3M", grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/client-body-buffer-size"])
			require.Len(t, grpcIngress.Spec.Rules, 1)
			require.Equal(t, "shared.example.test", grpcIngress.Spec.Rules[0].Host)
			require.Len(t, grpcIngress.Spec.Rules[0].HTTP.Paths, 2)
			for _, p := range grpcIngress.Spec.Rules[0].HTTP.Paths {
				require.Equal(t, serviceName, p.Backend.Service.Name)
				require.Equal(t, "http", p.Backend.Service.Port.Name)
			}
			require.Equal(t, "/cas.v1.ByteStream/", grpcIngress.Spec.Rules[0].HTTP.Paths[0].Path)
			require.Equal(t, "/cas.v1.ResourceService/", grpcIngress.Spec.Rules[0].HTTP.Paths[1].Path)

			svc := resources["Service/"+serviceName]
			require.Equal(t, tc.traefikScheme, svc.Metadata.Annotations["traefik.ingress.kubernetes.io/service.serversscheme"])
			casLegacy := resources["Ingress/"+tc.fullname+"-cas-api"]
			require.Equal(t, grpcBackend, casLegacy.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Len(t, casLegacy.Spec.Rules, 1)
			require.Equal(t, tc.fullname+"-cas-api", casLegacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		})
	}
}

type chartResource struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Rules []struct {
			Host string `yaml:"host"`
			HTTP struct {
				Paths []struct {
					Path    string `yaml:"path"`
					Backend struct {
						Service struct {
							Name string `yaml:"name"`
							Port struct {
								Name string `yaml:"name"`
							} `yaml:"port"`
						} `yaml:"service"`
					} `yaml:"backend"`
				} `yaml:"paths"`
			} `yaml:"http"`
		} `yaml:"rules"`
	} `yaml:"spec"`
}