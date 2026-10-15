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

package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
	"gopkg.in/yaml.v3"
)

const (
	chainloopYMLFile  = ".chainloop.yml"
	chainloopYAMLFile = ".chainloop.yaml"
)

// traceWorkflowName is the default workflow name used for trace attestations
// when no override is configured in .chainloop.yml.
const traceWorkflowName = "ai-coding-session"

// traceContractName is the workflow contract the trace workflow is attached to
// when the organization has it. It is the name of the library contract shipped
// with Chainloop, prefixed so it never collides with a contract of the user's
// own named after the workflow. Organizations without it get the control
// plane's default empty contract instead.
const traceContractName = "chainloop-ai-coding-session"

// ResolveWorkflowName returns the persisted workflow name when non-empty,
// otherwise falls back to the trace default. Centralizes the default so the
// CLI command and the hook handler stay in sync.
func ResolveWorkflowName(persisted string) string {
	if persisted != "" {
		return persisted
	}

	return traceWorkflowName
}

// ResolveContract returns the contract to attach when the trace workflow is
// created: the one given on the command line, or the trace default. required
// reports that the name came from the user, where a missing contract is an
// error instead of something to fall back from, because a typo must not
// silently bind another contract.
//
// Unlike the workflow name, the contract is not persisted to .chainloop.yml:
// it is only needed when the workflow is created, never on the push path.
func ResolveContract(flag string) (name string, required bool) {
	if flag != "" {
		return flag, true
	}

	return traceContractName, false
}

// SaveProjectToYML writes (or updates) the projectName field in
// .chainloop.yml (or .chainloop.yaml) at the given directory, preserving any other fields.
func SaveProjectToYML(dir, project string) error {
	return updateChainloopYMLField(dir, "projectName", project)
}

// SaveRequireTraceToYML writes (or updates) the requireTrace field in
// .chainloop.yml at the given directory, preserving any other fields.
//
// Deprecated: write traceRequired with SaveTraceRequiredToYML. This remains for
// the legacy path until requireTrace is dropped.
func SaveRequireTraceToYML(dir string, val bool) error {
	return updateChainloopYMLField(dir, "requireTrace", val)
}

const (
	// TraceStorageControlPlane uploads a signed attestation to the control
	// plane. It is the mode when traceStorage is unset.
	TraceStorageControlPlane = "CONTROL_PLANE"
	// TraceStorageLocal writes the session evidence set to disk, with no
	// control plane.
	TraceStorageLocal = "LOCAL"
)

// ResolveTraceStorage canonicalizes a traceStorage value: surrounding space is
// trimmed and the name upper-cased, an empty value is CONTROL_PLANE, and any
// value other than the two known modes is an error, so a committed typo surfaces
// instead of silently falling back to the control plane (and pushing to a
// backend a LOCAL repository means to avoid).
func ResolveTraceStorage(persisted string) (string, error) {
	switch v := strings.ToUpper(strings.TrimSpace(persisted)); v {
	case "":
		return TraceStorageControlPlane, nil
	case TraceStorageControlPlane, TraceStorageLocal:
		return v, nil
	default:
		return "", fmt.Errorf("unknown traceStorage %q in .chainloop.yml: use %s or %s", persisted, TraceStorageControlPlane, TraceStorageLocal)
	}
}

// LoadTraceStorageFromYML reads and canonicalizes the traceStorage mode from the
// nearest .chainloop.yml. A missing config or field is CONTROL_PLANE, but a
// config that exists and cannot be read or parsed is an error, never a silent
// CONTROL_PLANE. A LOCAL repository whose config broke must not have its
// pre-push hook fall back to the control plane and upload a local-only session.
func LoadTraceStorageFromYML(dir string) (string, error) {
	cfg, _, err := repositoryconfig.LoadChainloopYML(dir)
	if errors.Is(err, repositoryconfig.ErrChainloopYMLNotFound) {
		return TraceStorageControlPlane, nil
	}
	if err != nil {
		return "", err
	}

	return ResolveTraceStorage(cfg.TraceStorage)
}

// IsLocalStorage reports whether the repository is configured for LOCAL trace
// storage. It is best-effort: a missing config, or an invalid value, is treated
// as not-local, so a caller that only shapes local-mode behavior (a banner, a
// self-heal) falls back to the control-plane path. The pre-push hook calls
// LoadTraceStorageFromYML directly instead, so an invalid value blocks the push
// rather than being ignored.
func IsLocalStorage(dir string) bool {
	mode, err := LoadTraceStorageFromYML(dir)
	return err == nil && mode == TraceStorageLocal
}

// SaveTraceStorageToYML writes (or updates) the traceStorage field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveTraceStorageToYML(dir, mode string) error {
	return updateChainloopYMLField(dir, "traceStorage", mode)
}

// SaveTracePathToYML writes (or updates) the tracePath field in .chainloop.yml
// at the given directory, preserving any other fields.
func SaveTracePathToYML(dir, path string) error {
	return updateChainloopYMLField(dir, "tracePath", path)
}

// LoadTracePathFromYML looks for .chainloop.yml starting from dir and returns
// the tracePath value. Returns empty string when the field is absent.
func LoadTracePathFromYML(dir string) string {
	cfg := repositoryconfig.FindChainloopYML(dir)
	if cfg == nil {
		return ""
	}

	return cfg.TracePath
}

// SaveTraceRequiredToYML writes (or updates) the traceRequired field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveTraceRequiredToYML(dir string, val bool) error {
	return updateChainloopYMLField(dir, "traceRequired", val)
}

// LoadTraceRequiredFromYML reports whether a required trace must block a push.
// It prefers traceRequired and falls back to the deprecated requireTrace, so an
// older .chainloop.yml keeps working. Absent in both is false (default-off).
func LoadTraceRequiredFromYML(dir string) bool {
	cfg := repositoryconfig.FindChainloopYML(dir)
	if cfg == nil {
		return false
	}
	if cfg.TraceRequired != nil {
		return *cfg.TraceRequired
	}
	if cfg.RequireTrace != nil {
		return *cfg.RequireTrace
	}

	return false
}

// SaveOrganizationToYML writes (or updates) the organization field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveOrganizationToYML(dir, org string) error {
	return updateChainloopYMLField(dir, "organization", org)
}

// LoadOrganizationFromYML looks for .chainloop.yml starting from dir and returns
// the organization value. Returns empty string when the field is absent.
func LoadOrganizationFromYML(dir string) string {
	cfg := repositoryconfig.FindChainloopYML(dir)
	if cfg == nil {
		return ""
	}

	return cfg.Organization
}

// SaveWorkflowToYML writes (or updates) the workflowName field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveWorkflowToYML(dir, workflow string) error {
	return updateChainloopYMLField(dir, "workflowName", workflow)
}

// LoadWorkflowFromYML looks for .chainloop.yml starting from dir and returns
// the workflowName value. Returns empty string when the field is absent.
func LoadWorkflowFromYML(dir string) string {
	cfg := repositoryconfig.FindChainloopYML(dir)
	if cfg == nil {
		return ""
	}

	return cfg.WorkflowName
}

// updateChainloopYMLField reads, updates a single field, and writes back the
// .chainloop.yml file. The file is edited as a YAML node tree rather than
// re-marshalled from a map, so the user's comments, key order and indentation
// survive: .chainloop.yml is checked into their repository and a rewrite that
// reformats it turns `chainloop trace init` into a noisy diff.
func updateChainloopYMLField(dir, key string, value any) error {
	path := resolveChainloopYMLPath(dir)

	// A missing file is the same as an empty document: setYAMLField starts one.
	data, _ := os.ReadFile(path)

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}

	if err := setYAMLField(&doc, key, value); err != nil {
		return fmt.Errorf("update %s in %s: %w", key, filepath.Base(path), err)
	}

	out, err := encodeYAML(&doc, yamlIndent(data))
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}

	return os.WriteFile(path, out, 0600)
}

// encodeYAML renders a node tree with the given indentation.
func encodeYAML(doc *yaml.Node, indent int) ([]byte, error) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(indent)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

const (
	// defaultYAMLIndent is what .chainloop.yml uses; the encoder would
	// otherwise widen to 4.
	defaultYAMLIndent = 2
	// maxYAMLIndent bounds what is read as indentation, so a deeply indented
	// continuation line inside a block scalar cannot widen the whole file.
	maxYAMLIndent = 8
)

// yamlIndent reports the indentation width src already uses, taken from its
// first indented line, so nested blocks are written back the way the user
// wrote them rather than reindented to our own taste. Files with nothing
// nested (and new ones) get defaultYAMLIndent.
func yamlIndent(src []byte) int {
	for _, line := range strings.Split(string(src), "\n") {
		// YAML forbids tabs in indentation, so counting spaces is enough.
		body := strings.TrimLeft(line, " ")
		width := len(line) - len(body)
		if body == "" || width == 0 || width > maxYAMLIndent {
			continue
		}

		return width
	}

	return defaultYAMLIndent
}

// setYAMLField sets key to value in the document's top-level mapping, adding
// the key at the end when it is absent. An empty document (missing or empty
// file) is initialized to a mapping. Comments attached to an overwritten value
// are kept, since they document the field rather than the value.
func setYAMLField(doc *yaml.Node, key string, value any) error {
	var encoded yaml.Node
	if err := encoded.Encode(value); err != nil {
		return err
	}

	// A missing or empty file leaves the document node zero-valued.
	if len(doc.Content) == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}

	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a top-level YAML mapping")
	}

	// A mapping's Content alternates key, value.
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}

		old := mapping.Content[i+1]
		encoded.HeadComment = old.HeadComment
		encoded.LineComment = old.LineComment
		encoded.FootComment = old.FootComment
		mapping.Content[i+1] = &encoded

		return nil
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&encoded,
	)

	return nil
}

// LoadRequireTraceFromYML looks for .chainloop.yml starting from dir
// and returns the requireTrace value. Returns false when the field is
// absent or nil (default-off).
func LoadRequireTraceFromYML(dir string) bool {
	cfg := repositoryconfig.FindChainloopYML(dir)
	if cfg == nil || cfg.RequireTrace == nil {
		return false
	}

	return *cfg.RequireTrace
}

// ChainloopYMLName returns the base name of the Chainloop config file in dir:
// whichever of the two spellings is already there, or .chainloop.yml when
// neither is. Callers use it to name the file, for instance when asking the
// user to commit it.
func ChainloopYMLName(dir string) string {
	return filepath.Base(resolveChainloopYMLPath(dir))
}

// resolveChainloopYMLPath returns the path to the existing .chainloop.yml
// (or .chainloop.yaml) in dir. If neither exists, defaults to .chainloop.yml.
func resolveChainloopYMLPath(dir string) string {
	for _, name := range []string{chainloopYMLFile, chainloopYAMLFile} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(dir, chainloopYMLFile)
}
