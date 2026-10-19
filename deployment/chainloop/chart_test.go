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

// Verify the rendered resources, including templated F5 service names and
// overrides. The legacy gRPC resources keep their own upstreams.
func TestSharedIngress(t *testing.T) {
	for _, component := range []string{"controlplane", "cas"} {
		t.Run(component, func(t *testing.T) { testSharedIngress(t, component) })
	}
}

func testSharedIngress(t *testing.T, component string) {
	t.Helper()
	const (
		setFlag       = "--set"
		setStringFlag = "--set-string"
		grpcBackend   = "GRPC"
		h2cScheme     = "h2c"
		chartFullname = "test-chainloop"
	)
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is required for chart rendering tests")
	}
	for _, tc := range []struct {
		name            string
		args            []string
		fullname        string
		backendProtocol string
		traefikScheme   string
	}{
		{name: "defaults", fullname: chartFullname, backendProtocol: grpcBackend, traefikScheme: h2cScheme},
		{name: "custom name", args: []string{setFlag, "fullnameOverride=shared-api"}, fullname: "shared-api", backendProtocol: grpcBackend, traefikScheme: h2cScheme},
		{
			name:     "common annotations merge",
			args:     []string{setStringFlag, `commonAnnotations.nginx\.org/http2=false`},
			fullname: chartFullname, backendProtocol: grpcBackend, traefikScheme: h2cScheme,
		},
		{
			name: "encrypted upstream overrides",
			args: []string{
				setStringFlag, component + `.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPCS`,
				setStringFlag, component + `.service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=https`,
			},
			fullname: chartFullname, backendProtocol: "GRPCS", traefikScheme: "https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := tc.fullname + "-" + component
			args := append([]string{"template", "test", ".", setFlag, "development=true",
				setFlag, component + ".ingress.enabled=true", setFlag, component + ".ingress.hostname=shared.example.test",
				setFlag, "controlplane.ingressAPI.enabled=true", setFlag, "cas.ingressAPI.enabled=true"}, tc.args...)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, helm, args...)
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
			casName := tc.fullname + "-cas-api"
			cas := resources["Ingress/"+casName]
			require.Equal(t, grpcBackend, cas.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
			require.Len(t, cas.Spec.Rules, 1)
			require.Equal(t, casName, cas.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
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
