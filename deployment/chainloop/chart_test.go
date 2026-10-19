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
	"strings"
	"testing"
	"time"

	api "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
	"gopkg.in/yaml.v3"
)

const (
	casIngressEnabled = "cas.ingress.enabled=true"
	helmSet           = "--set"
	helmSetString     = "--set-string"
	annotationTrue    = "true"
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

func chartTemplate(t *testing.T, args []string) ([]byte, error) {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is required for chart rendering tests")
	}
	full := append([]string{"template", "test", ".", helmSet, "development=true",
		helmSet, "controlplane.ingressAPI.enabled=true", helmSet, "cas.ingressAPI.enabled=true"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helm, full...)
	// Local plugins are unrelated to chart rendering and can print
	// startup warnings on stdout before the YAML document.
	cmd.Env = append(os.Environ(), "HELM_PLUGINS="+t.TempDir())
	return cmd.CombinedOutput()
}

func renderChart(t *testing.T, args []string) map[string]chartResource {
	t.Helper()
	out, err := chartTemplate(t, args)
	require.NoError(t, err, "%s", out)
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
		{name: "custom name", args: []string{helmSet, "fullnameOverride=shared-api"}, fullname: "shared-api", backendProtocol: grpcBackend, traefikScheme: h2cScheme},
		{
			name:            "common annotations merge",
			args:            []string{helmSetString, `commonAnnotations.nginx\.org/http2=false`},
			fullname:        chartFullname,
			backendProtocol: grpcBackend, traefikScheme: h2cScheme,
		},
		{
			name: "encrypted upstream overrides",
			args: []string{
				helmSetString, `controlplane.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				helmSetString, `controlplane.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname: chartFullname, backendProtocol: "GRPCS", traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-controlplane"
			resources := renderChart(t, append(tc.args,
				helmSet, "controlplane.ingress.enabled=true", helmSet, "controlplane.ingress.hostname=shared.example.test"))
			ingress, ok := resources["Ingress/"+serviceName]
			require.True(t, ok)
			require.Equal(t, tc.backendProtocol, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Equal(t, serviceName, ingress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Equal(t, annotationTrue, ingress.Metadata.Annotations["nginx.org/http2"])
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
		{name: "defaults", fullname: chartFullname, grpcProtocol: grpcBackend, grpcHTTP2: annotationTrue},
		{name: "custom name", args: []string{helmSet, "fullnameOverride=shared-api"}, fullname: "shared-api", grpcProtocol: grpcBackend, grpcHTTP2: annotationTrue},
		{
			// A global annotation reaches both Ingress objects; on the gRPC
			// Ingress it overrides the chart default.
			name:         "common annotations merge",
			args:         []string{helmSetString, `commonAnnotations.nginx\.org/http2=false`},
			fullname:     chartFullname,
			grpcProtocol: grpcBackend, sharedHTTP2: "false", grpcHTTP2: "false",
		},
		{
			name: "encrypted upstream overrides",
			args: []string{
				helmSetString, `cas.ingress.grpc.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				helmSetString, `cas.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname:     chartFullname,
			grpcProtocol: "GRPCS", sharedHTTP2: "", grpcHTTP2: annotationTrue, traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-cas"
			resources := renderChart(t, append(tc.args,
				helmSet, casIngressEnabled, helmSet, "cas.ingress.hostname=shared.example.test"))

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
			require.Equal(t, annotationTrue, grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/service-upstream"])
			require.Empty(t, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/service-upstream"])
			require.Equal(t, serviceName, grpcIngress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Equal(t, tc.grpcHTTP2, grpcIngress.Metadata.Annotations["nginx.org/http2"])
			require.Equal(t, "100m", grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"])
			require.Equal(t, "3M", grpcIngress.Metadata.Annotations["nginx.ingress.kubernetes.io/client-body-buffer-size"])
			require.Len(t, grpcIngress.Spec.Rules, 1)
			require.Equal(t, "shared.example.test", grpcIngress.Spec.Rules[0].Host)
			require.Len(t, grpcIngress.Spec.Rules[0].HTTP.Paths, 8)
			for _, p := range grpcIngress.Spec.Rules[0].HTTP.Paths {
				require.Equal(t, serviceName, p.Backend.Service.Name)
				require.Equal(t, "grpc-shared", p.Backend.Service.Port.Name)
			}
			require.Equal(t, "/google.bytestream.ByteStream/", grpcIngress.Spec.Rules[0].HTTP.Paths[0].Path)
			require.Equal(t, "/cas.v1.ResourceService/", grpcIngress.Spec.Rules[0].HTTP.Paths[1].Path)

			svc := resources["Service/"+serviceName]
			require.Len(t, svc.Spec.Ports, 2)
			require.Equal(t, "http", svc.Spec.Ports[0].Name)
			require.Equal(t, "http", svc.Spec.Ports[0].TargetPort)
			require.Equal(t, "grpc-shared", svc.Spec.Ports[1].Name)
			require.Equal(t, 81, svc.Spec.Ports[1].Port)
			require.Equal(t, "http", svc.Spec.Ports[1].TargetPort)
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
		Ports []struct {
			Name       string `yaml:"name"`
			Port       int    `yaml:"port"`
			TargetPort string `yaml:"targetPort"`
		} `yaml:"ports"`
		TLS []struct {
			Hosts      []string `yaml:"hosts"`
			SecretName string   `yaml:"secretName"`
		} `yaml:"tls"`
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

func TestCASSharedIngressRPCPaths(t *testing.T) {
	resources := renderChart(t, []string{helmSet, casIngressEnabled})
	ingress := resources["Ingress/test-chainloop-cas-grpc"]
	// Derive the application routes from the generated descriptors, so a typo
	// in the values cannot become the expected result as well.
	for _, name := range []string{
		string(bytestream.File_google_bytestream_bytestream_proto.Services().Get(0).FullName()),
		api.ResourceService_ServiceDesc.ServiceName,
		api.StatusService_ServiceDesc.ServiceName,
		"grpc.health.v1.Health", "grpc.reflection.v1.ServerReflection",
		"grpc.reflection.v1alpha.ServerReflection", "kratos.api.Metadata",
		"grpc.channelz.v1.Channelz",
	} {
		covered := false
		for _, rule := range ingress.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				covered = covered || strings.HasPrefix("/"+name+"/", path.Path)
			}
		}
		require.True(t, covered, "no native gRPC route for %s", name)
	}
}

func TestCASSharedIngressTLS(t *testing.T) {
	for _, annotations := range []string{"cas.ingress.annotations", "cas.ingress.grpc.annotations", "commonAnnotations"} {
		t.Run(annotations, func(t *testing.T) {
			resources := renderChart(t, []string{helmSet, casIngressEnabled, helmSet, "cas.ingress.tls=true",
				helmSetString, annotations + `.cert-manager\.io/cluster-issuer=test-issuer`})
			owners := 0
			for _, name := range []string{"test-chainloop-cas", "test-chainloop-cas-grpc"} {
				ingress := resources["Ingress/"+name]
				if ingress.Metadata.Annotations["cert-manager.io/cluster-issuer"] != "" {
					owners++
				}
				require.Len(t, ingress.Spec.TLS, 1, "both routes must reference the certificate")
				require.Equal(t, "cas.dev.local-tls", ingress.Spec.TLS[0].SecretName)
				require.Equal(t, []string{"cas.dev.local"}, ingress.Spec.TLS[0].Hosts)
			}
			require.Equal(t, 1, owners, "only one Ingress should request the shared certificate")
		})
	}
}

func TestCASIngressInvalidPoolConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{name: "pod endpoints for both protocols", args: []string{helmSetString, `cas.ingress.grpc.annotations.nginx\.ingress\.kubernetes\.io/service-upstream=false`}, message: "requires separate ingress-nginx upstream addresses"},
		{name: "headless service", args: []string{helmSet, "cas.service.clusterIP=None"}, message: "requires a ClusterIP Service"},
		{name: "same service port", args: []string{helmSet, "cas.service.ports.grpcShared=80"}, message: "grpcShared must differ"},
		{name: "gRPC on HTTP routes", args: []string{helmSetString, `cas.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPC`}, message: "must use an HTTP/1.1 upstream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := chartTemplate(t, append(tc.args, helmSet, casIngressEnabled))
			require.Error(t, err)
			require.Contains(t, string(out), tc.message)
		})
	}
}

func TestCASSharedServicePortAlias(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		alias bool
	}{
		{name: "ingress disabled"},
		{name: "HTTP only", args: []string{helmSet, casIngressEnabled, "--set-json", "cas.ingress.grpc.paths=[]"}},
		{name: "both use ClusterIP", args: []string{helmSet, casIngressEnabled, helmSetString, `cas.ingress.annotations.nginx\.ingress\.kubernetes\.io/service-upstream=true`}, alias: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resources := renderChart(t, tc.args)
			ports := resources["Service/test-chainloop-cas"].Spec.Ports
			if tc.alias {
				require.Len(t, ports, 2)
			} else {
				require.Len(t, ports, 1)
			}
		})
	}
}
