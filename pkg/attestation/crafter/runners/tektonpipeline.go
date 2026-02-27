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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/rs/zerolog"
)

// Default paths for Kubernetes service account credentials
const (
	defaultSATokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultSANamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultSACACertPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Constants for Report() — writing attestation output to Tekton Results
const (
	defaultResultsDir      = "/tekton/results"
	tektonReportResultName = "attestation-report"
	maxTektonResultSize    = 3500
)

// podMetadata is a minimal struct for parsing K8s API pod response.
// Only the metadata.labels field is needed for Tekton label discovery.
type podMetadata struct {
	Metadata struct {
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
}

// TektonPipeline implements the SupportedRunner interface for Tekton Pipeline environments.
// It discovers Tekton metadata natively using a two-tier approach:
//   - Tier 1: HOSTNAME env var and SA namespace file (always available in K8s pods)
//   - Tier 2: K8s API pod labels for rich tekton.dev/* metadata (best-effort)
type TektonPipeline struct {
	ctx        context.Context
	logger     *zerolog.Logger
	podName    string            // from HOSTNAME env var
	namespace  string            // from /var/run/secrets/kubernetes.io/serviceaccount/namespace
	labels     map[string]string // tekton.dev/* labels from K8s API
	httpClient *http.Client      // injectable for testing
	resultsDir string            // default: "/tekton/results", injectable via WithResultsDir for testing

	// Injectable file paths for testing (defaults set in constructor)
	saTokenPath     string
	saNamespacePath string
	saCACertPath    string
}

// TektonPipelineOption is a functional option for configuring TektonPipeline.
type TektonPipelineOption func(*TektonPipeline)

// WithHTTPClient sets a custom HTTP client for K8s API calls.
// This is primarily used for testing with httptest.NewTLSServer.
func WithHTTPClient(client *http.Client) TektonPipelineOption {
	return func(t *TektonPipeline) { t.httpClient = client }
}

// WithSATokenPath overrides the default service account token file path.
func WithSATokenPath(path string) TektonPipelineOption {
	return func(t *TektonPipeline) { t.saTokenPath = path }
}

// WithNamespacePath overrides the default service account namespace file path.
func WithNamespacePath(path string) TektonPipelineOption {
	return func(t *TektonPipeline) { t.saNamespacePath = path }
}

// WithCACertPath overrides the default service account CA certificate file path.
func WithCACertPath(path string) TektonPipelineOption {
	return func(t *TektonPipeline) { t.saCACertPath = path }
}

// WithResultsDir overrides the default Tekton Results directory path.
// This is primarily used for testing Report() without requiring /tekton/results.
func WithResultsDir(dir string) TektonPipelineOption {
	return func(t *TektonPipeline) { t.resultsDir = dir }
}

// NewTektonPipeline creates a new TektonPipeline runner with two-tier native metadata discovery.
// ctx is used for K8s API calls; if nil, context.Background() is used as a safety fallback.
// The logger is required for debug-level logging of discovery failures.
// Functional options allow injecting test dependencies.
func NewTektonPipeline(ctx context.Context, logger *zerolog.Logger, opts ...TektonPipelineOption) *TektonPipeline {
	if ctx == nil {
		ctx = context.Background()
	}

	r := &TektonPipeline{
		ctx:             ctx,
		logger:          logger,
		labels:          make(map[string]string),
		saTokenPath:     defaultSATokenPath,
		saNamespacePath: defaultSANamespacePath,
		saCACertPath:    defaultSACACertPath,
		resultsDir:      defaultResultsDir,
	}

	// Apply functional options before discovery (allows test injection)
	for _, opt := range opts {
		opt(r)
	}

	// Tier 1: Always-available sources (no configuration required)
	r.podName = os.Getenv("HOSTNAME")

	if nsBytes, err := os.ReadFile(r.saNamespacePath); err == nil {
		r.namespace = strings.TrimSpace(string(nsBytes))
	} else {
		r.logger.Debug().Err(err).Msg("cannot read namespace file, namespace will be empty")
	}

	// Tier 2: K8s API for pod labels (best-effort, logs failures at debug level)
	r.discoverLabelsFromKubeAPI()

	return r
}

func (r *TektonPipeline) ID() schemaapi.CraftingSchema_Runner_RunnerType {
	return schemaapi.CraftingSchema_Runner_TEKTON_PIPELINE
}

// CheckEnv detects if we're running in a Tekton environment
// by checking for the existence of Tekton-specific directories
func (r *TektonPipeline) CheckEnv() bool {
	// Check for /tekton/results directory (most reliable indicator)
	if _, err := os.Stat("/tekton/results"); err == nil {
		return true
	}
	return false
}

func (r *TektonPipeline) ListEnvVars() []*EnvVarDefinition {
	// Tekton metadata is discovered natively from K8s API labels and filesystem.
	// These vars are "promoted" -- exposed to the attestation system even though the
	// values come from K8s API discovery, not from actual container env vars.
	// This follows the Dagger runner pattern of promoting encapsulated platform vars.
	return []*EnvVarDefinition{
		{"HOSTNAME", true},                  // optional (best-effort from env)
		{"TEKTON_TASKRUN_NAME", false},      // required (always in any TaskRun)
		{"TEKTON_TASK_NAME", true},          // optional (absent with inline taskSpec)
		{"TEKTON_NAMESPACE", false},         // required (always in any TaskRun)
		{"TEKTON_PIPELINE_NAME", true},      // optional (only Pipeline TaskRuns)
		{"TEKTON_PIPELINERUN_NAME", true},   // optional (only Pipeline TaskRuns)
		{"TEKTON_PIPELINE_TASK_NAME", true}, // optional (only Pipeline TaskRuns)
	}
}

func (r *TektonPipeline) RunURI() string {
	taskRunName := r.labels["tekton.dev/taskRun"]
	pipelineRunName := r.labels["tekton.dev/pipelineRun"]

	// Fallback: derive TaskRun name from HOSTNAME if K8s API labels unavailable
	if taskRunName == "" {
		taskRunName = r.taskRunNameFromHostname()
	}

	// Check for dashboard URL (opportunistic -- NOT required, NOT part of env var contract)
	dashboardURL := os.Getenv("TEKTON_DASHBOARD_URL")
	if dashboardURL != "" {
		dashboardURL = strings.TrimRight(dashboardURL, "/")
		// Prefer PipelineRun link if available
		if pipelineRunName != "" && r.namespace != "" {
			return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns/%s",
				dashboardURL, r.namespace, pipelineRunName)
		}
		if taskRunName != "" && r.namespace != "" {
			return fmt.Sprintf("%s/#/namespaces/%s/taskruns/%s",
				dashboardURL, r.namespace, taskRunName)
		}
	}

	// Fallback: construct a non-HTTP identifier URI for traceability
	if pipelineRunName != "" && r.namespace != "" {
		return fmt.Sprintf("tekton://%s/pipelineruns/%s", r.namespace, pipelineRunName)
	}
	if taskRunName != "" && r.namespace != "" {
		return fmt.Sprintf("tekton://%s/taskruns/%s", r.namespace, taskRunName)
	}

	return ""
}

// ResolveEnvVars returns internally-discovered metadata as key-value entries.
// Unlike other runners, this does NOT delegate to resolveEnvVars(r.ListEnvVars())
// because the real metadata comes from K8s API labels and filesystem, not from env vars.
// The returned keys (TEKTON_TASKRUN_NAME, etc.) are synthesized from discovered labels --
// they are NOT actual environment variables in the container.
//
// Required vars (TEKTON_TASKRUN_NAME, TEKTON_NAMESPACE) return errors if not resolved,
// blocking attestation. This forces proper RBAC configuration for pod get permissions.
// Optional vars (HOSTNAME, TEKTON_TASK_NAME, pipeline-specific labels) are silently skipped
// if empty. TEKTON_TASK_NAME is optional because inline taskSpec pipelines lack the
// tekton.dev/task pod label.
func (r *TektonPipeline) ResolveEnvVars() (map[string]string, []*error) {
	resolved := make(map[string]string)
	var errors []*error

	// requireVar adds the value to resolved if non-empty, or appends an error with RBAC hint.
	requireVar := func(key, value string) {
		if value != "" {
			resolved[key] = value
		} else {
			err := fmt.Errorf("required var %s not resolved -- ensure the ServiceAccount has pod get RBAC permissions", key)
			errors = append(errors, &err)
		}
	}

	// optionalVar adds the value to resolved if non-empty, silently skips otherwise.
	optionalVar := func(key, value string) {
		if value != "" {
			resolved[key] = value
		}
	}

	// HOSTNAME -- optional (best-effort from environment)
	optionalVar("HOSTNAME", os.Getenv("HOSTNAME"))

	// Required: always available in any TaskRun (needs RBAC for pod get)
	requireVar("TEKTON_TASKRUN_NAME", r.labels["tekton.dev/taskRun"])
	requireVar("TEKTON_NAMESPACE", r.namespace)

	// Optional: tekton.dev/task label is absent when Pipeline uses inline taskSpec
	optionalVar("TEKTON_TASK_NAME", r.labels["tekton.dev/task"])

	// Optional: only present in Pipeline-orchestrated TaskRuns
	optionalVar("TEKTON_PIPELINE_NAME", r.labels["tekton.dev/pipeline"])
	optionalVar("TEKTON_PIPELINERUN_NAME", r.labels["tekton.dev/pipelineRun"])
	optionalVar("TEKTON_PIPELINE_TASK_NAME", r.labels["tekton.dev/pipelineTask"])

	if len(errors) > 0 {
		return nil, errors
	}
	return resolved, nil
}

func (r *TektonPipeline) WorkflowFilePath() string {
	return ""
}

func (r *TektonPipeline) IsAuthenticated() bool {
	return false
}

// Environment detects managed K8s (GKE/EKS/AKS) vs self-hosted via cloud-provider env vars.
// These env vars are genuinely injected by the cloud platform when workload identity is configured,
// NOT by user configuration. Returns SelfHosted for plain K8s and Unknown if not in K8s at all.
func (r *TektonPipeline) Environment() RunnerEnvironment {
	// GKE with Workload Identity
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return Managed
	}

	// EKS with IRSA/Pod Identity
	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
		return Managed
	}

	// AKS with Workload Identity
	if os.Getenv("AZURE_FEDERATED_TOKEN_FILE") != "" {
		return Managed
	}

	// We know we're in K8s (CheckEnv passed), but can't determine managed vs self-hosted
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return SelfHosted
	}

	return Unknown
}

func (r *TektonPipeline) VerifyCommitSignature(_ context.Context, _ string) *commitverification.CommitVerification {
	return nil // Not supported for this runner
}

// Report writes attestation summary to Tekton Results with 3500-byte truncation.
// The Tekton Results system has a default max-result-size of 4096 bytes (shared with
// internal metadata), so we truncate at 3500 bytes to leave room for Tekton overhead.
func (r *TektonPipeline) Report(tableOutput []byte, attestationViewURL string) error {
	resultPath := filepath.Join(r.resultsDir, tektonReportResultName)

	content := fmt.Sprintf("Chainloop Attestation Report\n\n%s", tableOutput)
	if attestationViewURL != "" {
		content += fmt.Sprintf("\nView details: %s\n", attestationViewURL)
	}

	if len(content) > maxTektonResultSize {
		truncateAt := maxTektonResultSize - len("\n... (truncated)")
		content = content[:truncateAt] + "\n... (truncated)"
	}

	if err := os.WriteFile(resultPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write attestation report to Tekton Results: %w", err)
	}

	return nil
}

// taskRunNameFromHostname derives the TaskRun name from the pod HOSTNAME as a best-effort
// fallback when K8s API labels are unavailable. Tekton names pods as "<taskrun-name>-pod"
// (or "<taskrun-name>-pod-retryN" for retries). For long TaskRun names (>59 chars),
// kmeta.ChildName hashes the name, making hostname-to-taskrun derivation unreliable --
// in that case we return empty string.
func (r *TektonPipeline) taskRunNameFromHostname() string {
	hostname := r.podName
	if hostname == "" {
		return ""
	}
	// Handle retry suffix: <taskrun-name>-pod-retryN
	if idx := strings.Index(hostname, "-pod-retry"); idx != -1 {
		return hostname[:idx]
	}
	// Normal case: <taskrun-name>-pod
	if strings.HasSuffix(hostname, "-pod") {
		return strings.TrimSuffix(hostname, "-pod")
	}
	// Hashed name or unknown format -- can't reliably parse
	return ""
}

// discoverLabelsFromKubeAPI performs Tier 2 discovery by reading the pod's own labels
// from the Kubernetes API using the service account token. This is best-effort:
// if any step fails (missing SA token, RBAC denied, network error), it logs at debug
// level and returns without error. The runner continues with Tier 1 data only.
func (r *TektonPipeline) discoverLabelsFromKubeAPI() {
	// Read SA token
	token, err := os.ReadFile(r.saTokenPath)
	if err != nil {
		r.logger.Debug().Err(err).Msg("cannot read SA token, skipping K8s API discovery")
		return
	}

	// Read CA cert for TLS verification
	caCert, err := os.ReadFile(r.saCACertPath)
	if err != nil {
		r.logger.Debug().Err(err).Msg("cannot read CA cert, skipping K8s API discovery")
		return
	}

	// Build TLS config with cluster CA
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		r.logger.Debug().Msg("failed to parse CA cert, skipping K8s API discovery")
		return
	}

	// Create HTTP client with custom TLS if not injected (tests inject their own)
	if r.httpClient == nil {
		r.httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:    caCertPool,
					MinVersion: tls.VersionTLS12,
				},
			},
			Timeout: 5 * time.Second,
		}
	}

	// Build K8s API URL
	apiHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	apiPort := os.Getenv("KUBERNETES_SERVICE_PORT")
	if apiPort == "" {
		apiPort = "443"
	}

	if apiHost == "" || r.namespace == "" || r.podName == "" {
		r.logger.Debug().
			Str("apiHost", apiHost).
			Str("namespace", r.namespace).
			Str("podName", r.podName).
			Msg("missing required fields for K8s API discovery")
		return
	}

	url := fmt.Sprintf("https://%s:%s/api/v1/namespaces/%s/pods/%s",
		apiHost, apiPort, r.namespace, r.podName)

	// Add 10-second timeout for the K8s API call on top of parent context
	callCtx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, "GET", url, nil)
	if err != nil {
		r.logger.Debug().Err(err).Msg("failed to create K8s API request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.logger.Debug().Err(err).Msg("K8s API request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Debug().Int("status", resp.StatusCode).Msg("K8s API returned non-200")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.logger.Debug().Err(err).Msg("failed to read K8s API response body")
		return
	}

	var pod podMetadata
	if err := json.Unmarshal(body, &pod); err != nil {
		r.logger.Debug().Err(err).Msg("failed to parse pod metadata")
		return
	}

	// Filter labels with tekton.dev/ prefix
	for k, v := range pod.Metadata.Labels {
		if strings.HasPrefix(k, "tekton.dev/") {
			r.labels[k] = v
		}
	}

	r.logger.Debug().
		Int("labelCount", len(r.labels)).
		Msg("discovered Tekton labels from K8s API")
}
