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

const (
	casIngressEnabled = "cas.ingress.enabled=true"
	helmSet           = "--set"
	helmSetString     = "--set-string"
	annotationTrue    = "true"
)

// The control plane serves every protocol per request on its shared ingress
// (replacement spec D-001/D-002). The CAS does the same (spec D-020): the
// shared listener dispatches each request and reaches the native transport
// through an in-process reverse proxy, so one catch-all location with the
// gRPC backend protocol serves every request. The legacy gRPC resources keep
// their own upstreams.
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

// The CAS shared ingress mirrors the control plane (spec D-020): one
// catch-all location, the gRPC backend protocol for its HTTP/2 upstream, and
// the upload body limits. One Service port; no RPC path rules.
func testSharedCASIngress(t *testing.T) {
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
				helmSetString, `cas.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				helmSetString, `cas.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname: chartFullname, backendProtocol: "GRPCS", traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-cas"
			resources := renderChart(t, append(tc.args,
				helmSet, casIngressEnabled, helmSet, "cas.ingress.hostname=shared.example.test"))

			// One catch-all ingress serves every protocol on the shared
			// listener, over an HTTP/2 upstream.
			ingress, ok := resources["Ingress/"+serviceName]
			require.True(t, ok)
			require.Equal(t, tc.backendProtocol, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Equal(t, serviceName, ingress.Metadata.Annotations["nginx.org/grpc-services"])
			require.Equal(t, annotationTrue, ingress.Metadata.Annotations["nginx.org/http2"])
			require.Equal(t, "100m", ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"])
			require.Equal(t, "3M", ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/client-body-buffer-size"])
			require.Len(t, ingress.Spec.Rules, 1)
			require.Equal(t, "shared.example.test", ingress.Spec.Rules[0].Host)
			require.Len(t, ingress.Spec.Rules[0].HTTP.Paths, 1)
			path := ingress.Spec.Rules[0].HTTP.Paths[0]
			require.Equal(t, "/", path.Path)
			require.Equal(t, serviceName, path.Backend.Service.Name)
			require.Equal(t, "http", path.Backend.Service.Port.Name)

			// No second gRPC-path Ingress and no Service port alias remain.
			_, ok = resources["Ingress/"+serviceName+"-grpc"]
			require.False(t, ok)
			svc := resources["Service/"+serviceName]
			require.Len(t, svc.Spec.Ports, 1)
			require.Equal(t, "http", svc.Spec.Ports[0].Name)
			require.Equal(t, tc.traefikScheme, svc.Metadata.Annotations["traefik.ingress.kubernetes.io/service.serversscheme"])

			// The legacy gRPC ingress keeps its own upstream.
			casLegacy := resources["Ingress/"+tc.fullname+"-cas-api"]
			require.Equal(t, grpcBackend, casLegacy.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Len(t, casLegacy.Spec.Rules, 1)
			require.Equal(t, tc.fullname+"-cas-api", casLegacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		})
	}
}

type chartResource struct {
	Kind     string            `yaml:"kind"`
	Data     map[string]string `yaml:"data"`
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec chartPodSpec `yaml:"spec"`
		} `yaml:"template"`
		Ports []struct {
			Name        string `yaml:"name"`
			Port        int    `yaml:"port"`
			TargetPort  string `yaml:"targetPort"`
			AppProtocol string `yaml:"appProtocol"`
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

type chartPodSpec struct {
	Containers []struct {
		StartupProbe   chartProbe `yaml:"startupProbe"`
		LivenessProbe  chartProbe `yaml:"livenessProbe"`
		ReadinessProbe chartProbe `yaml:"readinessProbe"`
		VolumeMounts   []struct {
			Name      string `yaml:"name"`
			MountPath string `yaml:"mountPath"`
		} `yaml:"volumeMounts"`
	} `yaml:"containers"`
	Volumes []struct {
		Name   string `yaml:"name"`
		Secret *struct {
			SecretName string `yaml:"secretName"`
		} `yaml:"secret"`
	} `yaml:"volumes"`
}

type chartProbe struct {
	HTTPGet struct {
		Path   string `yaml:"path"`
		Port   string `yaml:"port"`
		Scheme string `yaml:"scheme"`
	} `yaml:"httpGet"`
}

func TestCASSharedIngressTLS(t *testing.T) {
	resources := renderChart(t, []string{helmSet, casIngressEnabled, helmSet, "cas.ingress.tls=true",
		helmSetString, `cas.ingress.annotations.cert-manager\.io/cluster-issuer=test-issuer`})
	ingress := resources["Ingress/test-chainloop-cas"]
	require.Equal(t, "test-issuer", ingress.Metadata.Annotations["cert-manager.io/cluster-issuer"])
	require.Len(t, ingress.Spec.TLS, 1)
	require.Equal(t, "cas.dev.local-tls", ingress.Spec.TLS[0].SecretName)
	require.Equal(t, []string{"cas.dev.local"}, ingress.Spec.TLS[0].Hosts)
}
