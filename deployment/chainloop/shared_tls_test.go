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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Pod TLS must agree across both listeners, certificate mounts, health probes,
// and proxy upstreams, independently of public ingress TLS termination.
func TestSharedListenerTLS(t *testing.T) {
	for _, component := range []string{"controlplane", "cas"} {
		t.Run(component, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				values     []string
				secret     string
				ingress    bool
				ingressTLS bool
				override   bool
			}{
				{name: "plaintext", ingress: true},
				{name: "existing secret without ingress", values: []string{"tls.existingSecret=server-tls"}, secret: "server-tls"},
				{name: "legacy alias without ingress", values: []string{"tlsConfig.secret.name=legacy-tls"}, secret: "legacy-tls"},
				{name: "existing secret with ingress", values: []string{"tls.existingSecret=server-tls"}, secret: "server-tls", ingress: true},
				{name: "legacy alias with ingress", values: []string{"tlsConfig.secret.name=legacy-tls"}, secret: "legacy-tls", ingress: true},
				{name: "existing secret takes precedence", values: []string{"tls.existingSecret=server-tls", "tlsConfig.secret.name=legacy-tls"}, secret: "server-tls", ingress: true},
				{name: "public TLS only", ingress: true, ingressTLS: true},
				{name: "public and pod TLS", values: []string{"tls.existingSecret=server-tls"}, secret: "server-tls", ingress: true, ingressTLS: true},
				{
					name: "explicit annotation overrides",
					values: []string{
						"tls.existingSecret=server-tls",
						`ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=GRPC`,
						`ingress.annotations.nginx\.org/ssl-services=custom-service`,
						`service.annotations.traefik\.ingress\.kubernetes\.io/service\.serversscheme=custom`,
					},
					secret: "server-tls", ingress: true, override: true,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					args := []string{}
					for _, value := range tc.values {
						args = append(args, helmSetString, component+"."+value)
					}
					if tc.ingress {
						args = append(args, helmSet, component+".ingress.enabled=true")
					}
					if tc.ingressTLS {
						args = append(args, helmSet, component+".ingress.tls=true",
							helmSetString, component+`.ingress.annotations.cert-manager\.io/cluster-issuer=test-issuer`)
					}
					resources := renderChart(t, args)
					name := "test-chainloop-" + component
					assertListenerTLS(t, resources, component, tc.secret)

					// Enabling TLS for one component must not enable it for the other.
					other := "cas"
					if component == other {
						other = "controlplane"
					}
					assertListenerTLS(t, resources, other, "")

					backend, scheme, sslService := "GRPC", "h2c", ""
					if tc.secret != "" {
						backend, scheme, sslService = "GRPCS", "https", name
					}
					if tc.override {
						backend, scheme, sslService = "GRPC", "custom", "custom-service"
					}
					require.Equal(t, scheme, resources["Service/"+name].Metadata.Annotations["traefik.ingress.kubernetes.io/service.serversscheme"])
					ingress, ok := resources["Ingress/"+name]
					require.Equal(t, tc.ingress, ok)
					if ok {
						require.Equal(t, backend, ingress.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
						require.Equal(t, name, ingress.Metadata.Annotations["nginx.org/grpc-services"])
						require.Equal(t, sslService, ingress.Metadata.Annotations["nginx.org/ssl-services"])
						require.Equal(t, tc.ingressTLS, len(ingress.Spec.TLS) > 0)
					}
					legacy := resources["Ingress/"+name+"-api"]
					require.Equal(t, "GRPC", legacy.Metadata.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
					require.Equal(t, name+"-api", legacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
				})
			}
		})
	}
}

func assertListenerTLS(t *testing.T, resources map[string]chartResource, component, secret string) {
	t.Helper()
	name := "test-chainloop-" + component
	protocol := "kubernetes.io/h2c"
	if secret != "" {
		protocol = "kubernetes.io/h2"
	}
	ports := resources["Service/"+name].Spec.Ports
	require.Len(t, ports, 1)
	require.Equal(t, "http", ports[0].Name)
	require.Equal(t, protocol, ports[0].AppProtocol)
	configKey := "config.yaml"
	if component == "cas" {
		configKey = "server.yaml"
	}
	var config struct {
		Server map[string]struct {
			TLSConfig map[string]string `yaml:"tls_config"`
		} `yaml:"server"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(resources["ConfigMap/"+name].Data[configKey]), &config))
	require.Contains(t, config.Server, "http")
	require.Contains(t, config.Server, "grpc")
	for _, listener := range []string{"http", "grpc"} {
		if secret == "" {
			require.Empty(t, config.Server[listener].TLSConfig)
		} else {
			require.Equal(t, map[string]string{
				"certificate": "/data/server-certs/tls.crt",
				"private_key": "/data/server-certs/tls.key",
			}, config.Server[listener].TLSConfig)
		}
	}
	require.Empty(t, config.Server["http_metrics"].TLSConfig)

	pod := resources["Deployment/"+name].Spec.Template.Spec
	require.NotEmpty(t, pod.Containers)
	container := pod.Containers[0]
	scheme := ""
	if secret != "" {
		scheme = "HTTPS"
	}
	for _, probe := range []chartProbe{container.StartupProbe, container.LivenessProbe, container.ReadinessProbe} {
		require.Equal(t, "http", probe.HTTPGet.Port)
		require.Equal(t, scheme, probe.HTTPGet.Scheme)
	}
	require.Equal(t, "/statusz?readiness=1", container.ReadinessProbe.HTTPGet.Path)
	mounts, volumes := 0, 0
	for _, mount := range container.VolumeMounts {
		if mount.Name == "server-certs" {
			mounts++
			require.Equal(t, "/data/server-certs", mount.MountPath)
		}
	}
	for _, volume := range pod.Volumes {
		if volume.Name == "server-certs" {
			volumes++
			require.NotNil(t, volume.Secret)
			require.Equal(t, secret, volume.Secret.SecretName)
		}
	}
	expected := 0
	if secret != "" {
		expected = 1
	}
	require.Equal(t, expected, mounts)
	require.Equal(t, expected, volumes)
}
