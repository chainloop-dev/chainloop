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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	notesResource        = "ConfigMap/rendered-notes"
	sharedEndpointsOptIn = "useLegacyGRPCEndpoints=false"
)

// renderWithNotes renders the chart and also NOTES.txt, which `helm template`
// skips. It copies the chart and wraps NOTES.txt in a ConfigMap template.
func renderWithNotes(t *testing.T, args []string) (map[string]chartResource, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(".")))
	templates, err := os.OpenRoot(filepath.Join(dir, "templates"))
	require.NoError(t, err)
	defer templates.Close()

	notes, err := templates.ReadFile("NOTES.txt")
	require.NoError(t, err)
	require.NoError(t, templates.Remove("NOTES.txt"))
	tpl := `{{- define "test.notes" -}}` + string(notes) + `{{- end -}}`
	require.NoError(t, templates.WriteFile("_test_notes.tpl", []byte(tpl), 0o600))
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: rendered-notes\ndata:\n  notes: |\n{{ include \"test.notes\" . | indent 4 }}\n"
	require.NoError(t, templates.WriteFile("zz-test-notes.yaml", []byte(manifest), 0o600))

	t.Chdir(dir)
	resources := renderChart(t, args)
	rendered, ok := resources[notesResource]
	require.True(t, ok)
	return resources, rendered.Data["notes"]
}

type controlplaneConfig struct {
	CASServer struct {
		GRPC struct {
			Addr string `yaml:"addr"`
		} `yaml:"grpc"`
	} `yaml:"cas_server"`
}

// useLegacyGRPCEndpoints selects the endpoints that the clients use: the
// control plane connection to the CAS and the CLI configuration in NOTES.
// Only an explicit false selects the shared endpoints.
func TestUseLegacyGRPCEndpoints(t *testing.T) {
	const (
		legacyCASAddr = "test-chainloop-cas-api:80"
		sharedCASAddr = "test-chainloop-cas:80"
	)
	sharedIngresses := []string{
		helmSet, "controlplane.ingress.enabled=true", helmSet, "controlplane.ingress.tls=true",
		helmSet, casIngressEnabled, helmSet, "cas.ingress.tls=true",
		helmSet, "controlplane.ingressAPI.tls=true", helmSet, "cas.ingressAPI.tls=true",
	}
	for _, tc := range []struct {
		name      string
		args      []string
		casAddr   string
		wantNotes []string
	}{
		{
			name:    "default uses the legacy endpoints",
			args:    sharedIngresses,
			casAddr: legacyCASAddr,
			wantNotes: []string{
				"chainloop config save \\\n    --control-plane api.cp.dev.local:443 \\\n    --artifact-cas api.cas.dev.local:443",
			},
		},
		{
			name:    "explicit true uses the legacy endpoints",
			args:    append([]string{helmSet, "useLegacyGRPCEndpoints=true"}, sharedIngresses...),
			casAddr: legacyCASAddr,
			wantNotes: []string{
				"--control-plane api.cp.dev.local:443 \\\n    --artifact-cas api.cas.dev.local:443",
			},
		},
		{
			// helm upgrade --reuse-values from an older chart leaves the key unset.
			name:    "unset key uses the legacy endpoints",
			args:    append([]string{helmSet, "useLegacyGRPCEndpoints=null"}, sharedIngresses...),
			casAddr: legacyCASAddr,
			wantNotes: []string{
				"--control-plane api.cp.dev.local:443 \\\n    --artifact-cas api.cas.dev.local:443",
			},
		},
		{
			name:    "false uses the shared endpoints",
			args:    append([]string{helmSet, sharedEndpointsOptIn}, sharedIngresses...),
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"chainloop config save \\\n    --control-plane cp.dev.local:443 \\\n    --artifact-cas cas.dev.local:443",
			},
		},
		{
			name:    "false as a string uses the shared endpoints",
			args:    append([]string{helmSetString, sharedEndpointsOptIn}, sharedIngresses...),
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"--control-plane cp.dev.local:443 \\\n    --artifact-cas cas.dev.local:443",
			},
		},
		{
			name:    "false without shared TLS keeps the CLI insecure",
			args:    []string{helmSet, sharedEndpointsOptIn, helmSet, "controlplane.ingress.enabled=true", helmSet, casIngressEnabled},
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"chainloop --insecure config save \\\n    --control-plane cp.dev.local:80 \\\n    --artifact-cas cas.dev.local:80",
			},
		},
		{
			name:    "false without shared ingresses port-forwards the shared services",
			args:    []string{helmSet, sharedEndpointsOptIn},
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"svc/test-chainloop-controlplane 9000:80",
				"svc/test-chainloop-cas 9001:80",
				"--control-plane localhost:9000 \\\n       --artifact-cas localhost:9001",
			},
		},
		{
			name: "false with NodePort uses the shared service node ports",
			args: []string{
				helmSet, sharedEndpointsOptIn,
				helmSet, "controlplane.ingressAPI.enabled=false", helmSet, "cas.ingressAPI.enabled=false",
				helmSet, "controlplane.service.type=NodePort", helmSetString, "controlplane.service.nodePorts.http=30800",
				helmSet, "cas.service.type=NodePort", helmSetString, "cas.service.nodePorts.http=30801",
			},
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"--control-plane localhost:30800 \\\n    --artifact-cas localhost:30801",
			},
		},
		{
			name: "false with LoadBalancer reads the shared services",
			args: []string{
				helmSet, sharedEndpointsOptIn,
				helmSet, "controlplane.ingressAPI.enabled=false", helmSet, "cas.ingressAPI.enabled=false",
				helmSet, "controlplane.service.type=LoadBalancer", helmSet, "cas.service.type=LoadBalancer",
			},
			casAddr: sharedCASAddr,
			wantNotes: []string{
				"svc test-chainloop-controlplane -w",
				"svc test-chainloop-cas -w",
				"--control-plane $CP_HOST:$CP_PORT \\\n       --artifact-cas $CAS_HOST:$CAS_PORT",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resources, notes := renderWithNotes(t, tc.args)
			for _, want := range tc.wantNotes {
				require.Contains(t, notes, want)
			}

			var cfg controlplaneConfig
			require.NoError(t, yaml.Unmarshal([]byte(resources["ConfigMap/test-chainloop-controlplane"].Data["config.yaml"]), &cfg))
			require.Equal(t, tc.casAddr, cfg.CASServer.GRPC.Addr)

			// Both modes keep the legacy gRPC resources.
			_, ok := resources["Service/test-chainloop-controlplane-api"]
			require.True(t, ok)
			_, ok = resources["Service/test-chainloop-cas-api"]
			require.True(t, ok)
		})
	}
}
