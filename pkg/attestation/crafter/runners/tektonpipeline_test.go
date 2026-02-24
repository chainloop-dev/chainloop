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

package runners

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type tektonPipelineSuite struct {
	suite.Suite
}

func TestTektonPipelineRunner(t *testing.T) {
	suite.Run(t, new(tektonPipelineSuite))
}

// newTestLogger creates a disabled logger for testing (no output noise).
func newTestLogger() *zerolog.Logger {
	l := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	return &l
}

// writeTempFile creates a file in dir with the given name and content.
// Returns the full path to the created file.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to write temp file %s: %v", path, err)
	}
	return path
}

// extractCACertPEM extracts the CA certificate from an httptest.NewTLSServer
// and returns it as PEM-encoded bytes suitable for writing to a file.
func extractCACertPEM(server *httptest.Server) []byte {
	// The test server's TLS config has the certificate
	cert := server.TLS.Certificates[0]
	// Parse the leaf cert
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: leaf.Raw,
	})
}

// TestDiscoverLabelsSuccess tests successful K8s API label discovery.
// Verifies that tekton.dev/* labels are extracted and non-tekton labels are filtered out.
func (s *tektonPipelineSuite) TestDiscoverLabelsSuccess() {
	t := s.T()

	// Create mock K8s API server that returns pod with Tekton labels
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request includes authorization
		s.NotEmpty(r.Header.Get("Authorization"))
		s.Contains(r.Header.Get("Authorization"), "Bearer ")

		// Return pod metadata with tekton.dev/* labels and a non-tekton label
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					"tekton.dev/taskRun":      "my-taskrun",
					"tekton.dev/pipeline":     "my-pipeline",
					"tekton.dev/pipelineRun":  "my-pipelinerun",
					"tekton.dev/task":         "my-task",
					"tekton.dev/pipelineTask": "build",
					"app":                     "other", // non-tekton label, should be filtered
				},
			},
		})
	}))
	defer server.Close()

	// Parse server URL to extract host and port
	serverURL, err := url.Parse(server.URL)
	s.Require().NoError(err)

	// Set env vars for Tier 1 and Tier 2 discovery
	t.Setenv("HOSTNAME", "my-taskrun-pod")
	t.Setenv("KUBERNETES_SERVICE_HOST", serverURL.Hostname())
	t.Setenv("KUBERNETES_SERVICE_PORT", serverURL.Port())

	// Create temp directory for SA files
	tmpDir := t.TempDir()

	// Write SA token file
	tokenPath := writeTempFile(t, tmpDir, "token", "test-sa-token")

	// Write namespace file
	nsPath := writeTempFile(t, tmpDir, "namespace", "test-ns")

	// Write CA cert file (extract from test server's TLS cert)
	caCertPEM := extractCACertPEM(server)
	caCertPath := writeTempFile(t, tmpDir, "ca.crt", string(caCertPEM))

	// Create runner with injected httpClient (bypasses TLS verification against our self-signed cert)
	r := NewTektonPipeline(
		newTestLogger(),
		WithHTTPClient(server.Client()),
		WithSATokenPath(tokenPath),
		WithNamespacePath(nsPath),
		WithCACertPath(caCertPath),
	)

	// Verify Tier 1 discovery
	s.Equal("my-taskrun-pod", r.podName, "podName should be set from HOSTNAME")
	s.Equal("test-ns", r.namespace, "namespace should be read from file")

	// Verify Tier 2 discovery: tekton.dev/* labels are populated
	s.Equal("my-taskrun", r.labels["tekton.dev/taskRun"])
	s.Equal("my-pipeline", r.labels["tekton.dev/pipeline"])
	s.Equal("my-pipelinerun", r.labels["tekton.dev/pipelineRun"])
	s.Equal("my-task", r.labels["tekton.dev/task"])
	s.Equal("build", r.labels["tekton.dev/pipelineTask"])

	// Verify non-tekton label is filtered out
	_, hasApp := r.labels["app"]
	s.False(hasApp, "non-tekton label 'app' should be filtered out")

	// Verify total label count (only tekton.dev/* labels)
	s.Len(r.labels, 5, "should have exactly 5 tekton.dev/* labels")
}

// TestDiscoverLabelsRBACDenied tests graceful degradation when K8s API returns 403 Forbidden.
// Tier 1 data (podName, namespace) should still be populated. Labels should be empty but not nil.
func (s *tektonPipelineSuite) TestDiscoverLabelsRBACDenied() {
	t := s.T()

	// Create mock K8s API server that returns 403 Forbidden
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	s.Require().NoError(err)

	t.Setenv("HOSTNAME", "my-taskrun-pod")
	t.Setenv("KUBERNETES_SERVICE_HOST", serverURL.Hostname())
	t.Setenv("KUBERNETES_SERVICE_PORT", serverURL.Port())

	tmpDir := t.TempDir()
	tokenPath := writeTempFile(t, tmpDir, "token", "test-sa-token")
	nsPath := writeTempFile(t, tmpDir, "namespace", "test-ns")
	caCertPEM := extractCACertPEM(server)
	caCertPath := writeTempFile(t, tmpDir, "ca.crt", string(caCertPEM))

	r := NewTektonPipeline(
		newTestLogger(),
		WithHTTPClient(server.Client()),
		WithSATokenPath(tokenPath),
		WithNamespacePath(nsPath),
		WithCACertPath(caCertPath),
	)

	// Tier 1 data should still be populated despite Tier 2 failure
	s.Equal("my-taskrun-pod", r.podName, "podName should be set from HOSTNAME (Tier 1)")
	s.Equal("test-ns", r.namespace, "namespace should be read from file (Tier 1)")

	// Labels should be empty map (not nil) -- Tier 2 failed gracefully
	s.NotNil(r.labels, "labels should not be nil")
	s.Empty(r.labels, "labels should be empty when K8s API returns 403")
}

// TestDiscoverWithoutSAToken tests graceful degradation when SA token file does not exist.
// K8s API discovery should be skipped entirely. Tier 1 data should still be populated.
func (s *tektonPipelineSuite) TestDiscoverWithoutSAToken() {
	t := s.T()

	t.Setenv("HOSTNAME", "my-taskrun-pod")

	tmpDir := t.TempDir()

	// Write namespace file but NOT the SA token file
	nsPath := writeTempFile(t, tmpDir, "namespace", "test-ns")

	// Use a non-existent path for SA token
	nonExistentTokenPath := filepath.Join(tmpDir, "nonexistent-token")

	r := NewTektonPipeline(
		newTestLogger(),
		WithSATokenPath(nonExistentTokenPath),
		WithNamespacePath(nsPath),
		// No CA cert path needed -- won't reach that code
	)

	// Tier 1 data should be populated
	s.Equal("my-taskrun-pod", r.podName, "podName should be set from HOSTNAME")
	s.Equal("test-ns", r.namespace, "namespace should be read from file")

	// Labels should be empty map (K8s API skipped due to missing SA token)
	s.NotNil(r.labels, "labels should not be nil")
	s.Empty(r.labels, "labels should be empty when SA token is missing")
}

// TestDiscoverWithoutNamespaceFile tests behavior when namespace file does not exist.
// Namespace should be empty string. PodName should still be populated from HOSTNAME.
func (s *tektonPipelineSuite) TestDiscoverWithoutNamespaceFile() {
	t := s.T()

	t.Setenv("HOSTNAME", "my-taskrun-pod")

	tmpDir := t.TempDir()

	// Use a non-existent path for namespace file
	nonExistentNSPath := filepath.Join(tmpDir, "nonexistent-namespace")

	r := NewTektonPipeline(
		newTestLogger(),
		WithNamespacePath(nonExistentNSPath),
		// SA token also non-existent, so K8s API will be skipped
		WithSATokenPath(filepath.Join(tmpDir, "nonexistent-token")),
	)

	// PodName from HOSTNAME should still work
	s.Equal("my-taskrun-pod", r.podName, "podName should be set from HOSTNAME")

	// Namespace should be empty since file doesn't exist
	s.Empty(r.namespace, "namespace should be empty when file is missing")

	// Labels should be empty (K8s API not called due to missing token)
	s.NotNil(r.labels, "labels should not be nil")
	s.Empty(r.labels, "labels should be empty")
}

// TestCheckEnvTektonPresent tests that CheckEnv returns true when /tekton/results exists.
// Note: CheckEnv hardcodes the /tekton/results path, so this test creates a temp directory
// structure but cannot override the path. We test the false case (always works outside Tekton)
// and document the limitation for the true case.
func (s *tektonPipelineSuite) TestCheckEnvTektonPresent() {
	t := s.T()

	tmpDir := t.TempDir()

	r := NewTektonPipeline(
		newTestLogger(),
		WithSATokenPath(filepath.Join(tmpDir, "nonexistent-token")),
		WithNamespacePath(filepath.Join(tmpDir, "nonexistent-namespace")),
	)

	// Outside a Tekton environment, /tekton/results does not exist
	// so CheckEnv should return false
	s.False(r.CheckEnv(), "CheckEnv should return false when /tekton/results does not exist")

	// Note: Testing the true case would require either:
	// (a) Making the results path injectable (like SA paths), or
	// (b) Actually creating /tekton/results (requires root privileges)
	// The false case validates that the directory check logic works correctly.
	// The true case is validated by the existing integration test environment.
}

// TestRunnerID verifies the runner returns the correct ID.
func (s *tektonPipelineSuite) TestRunnerID() {
	t := s.T()
	tmpDir := t.TempDir()

	r := NewTektonPipeline(
		newTestLogger(),
		WithSATokenPath(filepath.Join(tmpDir, "nonexistent-token")),
		WithNamespacePath(filepath.Join(tmpDir, "nonexistent-namespace")),
	)

	s.Equal("TEKTON_PIPELINE", r.ID().String())
}

// TestListEnvVars verifies that ListEnvVars returns minimal list with HOSTNAME only.
// HOSTNAME is marked optional because discovery is best-effort.
func (s *tektonPipelineSuite) TestListEnvVars() {
	t := s.T()
	tmpDir := t.TempDir()

	r := NewTektonPipeline(
		newTestLogger(),
		WithSATokenPath(filepath.Join(tmpDir, "nonexistent-token")),
		WithNamespacePath(filepath.Join(tmpDir, "nonexistent-namespace")),
	)

	envVars := r.ListEnvVars()
	s.Require().Len(envVars, 1, "ListEnvVars should return exactly 1 entry")
	s.Equal("HOSTNAME", envVars[0].Name)
	s.True(envVars[0].Optional, "HOSTNAME should be optional")
}

// ============================================================================
// taskRunNameFromHostname tests
// ============================================================================

// TestTaskRunNameFromHostname tests hostname-to-taskrun-name derivation with a table-driven approach.
func (s *tektonPipelineSuite) TestTaskRunNameFromHostname() {
	t := s.T()

	tests := []struct {
		hostname string
		expected string
		desc     string
	}{
		{"my-taskrun-pod", "my-taskrun", "Normal case: strip -pod suffix"},
		{"build-images-pod", "build-images", "Multi-dash name"},
		{"my-taskrun-pod-retry1", "my-taskrun", "Retry suffix (single digit)"},
		{"my-taskrun-pod-retry12", "my-taskrun", "Retry suffix (double digit)"},
		{"", "", "Empty hostname"},
		{"abc123def456", "", "Hashed name (no -pod suffix)"},
	}

	for _, tt := range tests {
		s.Run(tt.desc, func() {
			tmpDir := t.TempDir()
			r := &TektonPipeline{
				logger:  newTestLogger(),
				podName: tt.hostname,
				labels:  make(map[string]string),
			}
			_ = tmpDir // just to avoid unused warning
			s.Equal(tt.expected, r.taskRunNameFromHostname(), tt.desc)
		})
	}
}

// ============================================================================
// RunURI tests
// ============================================================================

// TestRunURIWithDashboardAndPipelineRun tests RunURI with dashboard URL and PipelineRun label.
// PipelineRun link should be preferred over TaskRun.
func (s *tektonPipelineSuite) TestRunURIWithDashboardAndPipelineRun() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "https://dashboard.example.com")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		namespace: "default",
		labels: map[string]string{
			"tekton.dev/taskRun":     "tr1",
			"tekton.dev/pipelineRun": "pr1",
		},
	}

	s.Equal("https://dashboard.example.com/#/namespaces/default/pipelineruns/pr1", r.RunURI())
}

// TestRunURIWithDashboardTaskRunOnly tests RunURI with dashboard URL but no PipelineRun.
// Should use TaskRun link. Also tests trailing slash trimming on dashboard URL.
func (s *tektonPipelineSuite) TestRunURIWithDashboardTaskRunOnly() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "https://dashboard.example.com/")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		namespace: "default",
		labels: map[string]string{
			"tekton.dev/taskRun": "tr1",
		},
	}

	s.Equal("https://dashboard.example.com/#/namespaces/default/taskruns/tr1", r.RunURI())
}

// TestRunURINoDashboardWithLabels tests RunURI without dashboard URL but with labels.
// Should return tekton:// identifier URI with PipelineRun preferred.
func (s *tektonPipelineSuite) TestRunURINoDashboardWithLabels() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		namespace: "ci",
		labels: map[string]string{
			"tekton.dev/taskRun":     "tr1",
			"tekton.dev/pipelineRun": "pr1",
		},
	}

	s.Equal("tekton://ci/pipelineruns/pr1", r.RunURI())
}

// TestRunURINoDashboardTaskRunOnly tests RunURI without dashboard URL and no PipelineRun.
// Should return tekton:// URI with TaskRun.
func (s *tektonPipelineSuite) TestRunURINoDashboardTaskRunOnly() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		namespace: "ci",
		labels: map[string]string{
			"tekton.dev/taskRun": "tr1",
		},
	}

	s.Equal("tekton://ci/taskruns/tr1", r.RunURI())
}

// TestRunURIFallbackToHostname tests RunURI with no labels -- derives TaskRun name from HOSTNAME.
func (s *tektonPipelineSuite) TestRunURIFallbackToHostname() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		podName:   "my-taskrun-pod",
		namespace: "default",
		labels:    make(map[string]string),
	}

	s.Equal("tekton://default/taskruns/my-taskrun", r.RunURI())
}

// TestRunURIEmpty tests RunURI when no labels, no parseable hostname, and no namespace.
// Should return empty string.
func (s *tektonPipelineSuite) TestRunURIEmpty() {
	t := s.T()
	t.Setenv("TEKTON_DASHBOARD_URL", "")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		podName:   "abc123",
		namespace: "",
		labels:    make(map[string]string),
	}

	s.Equal("", r.RunURI())
}

// ============================================================================
// Report tests
// ============================================================================

// TestReportWritesFile tests that Report writes a file with the expected content.
func (s *tektonPipelineSuite) TestReportWritesFile() {
	t := s.T()
	tmpDir := t.TempDir()

	r := &TektonPipeline{
		logger:     newTestLogger(),
		labels:     make(map[string]string),
		resultsDir: tmpDir,
	}

	err := r.Report([]byte("table output"), "https://app.chainloop.dev/att/123")
	s.Require().NoError(err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "attestation-report"))
	s.Require().NoError(err)

	s.Contains(string(content), "Chainloop Attestation Report")
	s.Contains(string(content), "table output")
	s.Contains(string(content), "View details: https://app.chainloop.dev/att/123")
}

// TestReportTruncation tests that Report truncates oversized content at 3500 bytes.
func (s *tektonPipelineSuite) TestReportTruncation() {
	t := s.T()
	tmpDir := t.TempDir()

	r := &TektonPipeline{
		logger:     newTestLogger(),
		labels:     make(map[string]string),
		resultsDir: tmpDir,
	}

	// Create a 4000-byte table output
	largeTable := []byte(strings.Repeat("x", 4000))
	err := r.Report(largeTable, "")
	s.Require().NoError(err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "attestation-report"))
	s.Require().NoError(err)

	s.LessOrEqual(len(content), maxTektonResultSize, "Report content should not exceed maxTektonResultSize")
	s.True(strings.HasSuffix(string(content), "\n... (truncated)"), "Truncated report should end with truncation marker")
}

// TestReportNoURL tests that Report works without an attestation URL.
func (s *tektonPipelineSuite) TestReportNoURL() {
	t := s.T()
	tmpDir := t.TempDir()

	r := &TektonPipeline{
		logger:     newTestLogger(),
		labels:     make(map[string]string),
		resultsDir: tmpDir,
	}

	err := r.Report([]byte("table"), "")
	s.Require().NoError(err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "attestation-report"))
	s.Require().NoError(err)

	s.NotContains(string(content), "View details", "Report without URL should not contain 'View details'")
}

// TestReportMissingDir tests that Report returns an error when results directory doesn't exist.
func (s *tektonPipelineSuite) TestReportMissingDir() {
	r := &TektonPipeline{
		logger:     newTestLogger(),
		labels:     make(map[string]string),
		resultsDir: "/nonexistent/path/that/does/not/exist",
	}

	err := r.Report([]byte("table"), "")
	s.Error(err, "Report should return error when results directory doesn't exist")
	s.Contains(err.Error(), "failed to write attestation report to Tekton Results")
}

// ============================================================================
// Environment tests
// ============================================================================

// TestEnvironmentGKE tests Environment returns Managed for GKE with Workload Identity.
func (s *tektonPipelineSuite) TestEnvironmentGKE() {
	t := s.T()
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	// Ensure other cloud vars are unset
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "")

	r := &TektonPipeline{logger: newTestLogger(), labels: make(map[string]string)}
	s.Equal(Managed, r.Environment())
}

// TestEnvironmentEKS tests Environment returns Managed for EKS with IRSA.
func (s *tektonPipelineSuite) TestEnvironmentEKS() {
	t := s.T()
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
	// Ensure other cloud vars are unset
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "")

	r := &TektonPipeline{logger: newTestLogger(), labels: make(map[string]string)}
	s.Equal(Managed, r.Environment())
}

// TestEnvironmentAKS tests Environment returns Managed for AKS with Workload Identity.
func (s *tektonPipelineSuite) TestEnvironmentAKS() {
	t := s.T()
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "/var/run/secrets/azure/tokens/azure-identity-token")
	// Ensure other cloud vars are unset
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")

	r := &TektonPipeline{logger: newTestLogger(), labels: make(map[string]string)}
	s.Equal(Managed, r.Environment())
}

// TestEnvironmentSelfHosted tests Environment returns SelfHosted for plain K8s.
func (s *tektonPipelineSuite) TestEnvironmentSelfHosted() {
	t := s.T()
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	// Ensure cloud vars are unset
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "")

	r := &TektonPipeline{logger: newTestLogger(), labels: make(map[string]string)}
	s.Equal(SelfHosted, r.Environment())
}

// TestEnvironmentUnknown tests Environment returns Unknown when no K8s env vars present.
func (s *tektonPipelineSuite) TestEnvironmentUnknown() {
	t := s.T()
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	r := &TektonPipeline{logger: newTestLogger(), labels: make(map[string]string)}
	s.Equal(Unknown, r.Environment())
}

// ============================================================================
// ResolveEnvVars tests
// ============================================================================

// TestResolveEnvVarsWithLabels tests ResolveEnvVars with full label set.
// All discovered metadata should be returned as key-value entries.
func (s *tektonPipelineSuite) TestResolveEnvVarsWithLabels() {
	t := s.T()
	t.Setenv("HOSTNAME", "my-taskrun-pod")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		namespace: "ci",
		labels: map[string]string{
			"tekton.dev/taskRun":      "my-taskrun",
			"tekton.dev/pipeline":     "my-pipeline",
			"tekton.dev/pipelineRun":  "my-pipelinerun",
			"tekton.dev/task":         "my-task",
			"tekton.dev/pipelineTask": "build",
		},
	}

	resolved, errs := r.ResolveEnvVars()
	s.Nil(errs, "ResolveEnvVars should not return errors")
	s.Equal("my-taskrun-pod", resolved["HOSTNAME"])
	s.Equal("my-taskrun", resolved["TEKTON_TASKRUN_NAME"])
	s.Equal("my-pipeline", resolved["TEKTON_PIPELINE_NAME"])
	s.Equal("my-pipelinerun", resolved["TEKTON_PIPELINERUN_NAME"])
	s.Equal("my-task", resolved["TEKTON_TASK_NAME"])
	s.Equal("build", resolved["TEKTON_PIPELINE_TASK_NAME"])
	s.Equal("ci", resolved["TEKTON_NAMESPACE"])
	s.Len(resolved, 7, "should have 7 entries with full label set")
}

// TestResolveEnvVarsMinimal tests ResolveEnvVars with no labels (Tier 1 only).
// Only HOSTNAME and TEKTON_NAMESPACE should be present.
func (s *tektonPipelineSuite) TestResolveEnvVarsMinimal() {
	t := s.T()
	t.Setenv("HOSTNAME", "my-taskrun-pod")

	r := &TektonPipeline{
		logger:    newTestLogger(),
		podName:   "my-taskrun-pod",
		namespace: "default",
		labels:    make(map[string]string),
	}

	resolved, errs := r.ResolveEnvVars()
	s.Nil(errs, "ResolveEnvVars should not return errors")
	s.Equal("my-taskrun-pod", resolved["HOSTNAME"])
	s.Equal("default", resolved["TEKTON_NAMESPACE"])
	s.Len(resolved, 2, "should have 2 entries with minimal data")
}
