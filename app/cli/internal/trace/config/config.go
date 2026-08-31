package config

import (
	"fmt"
	"os"
	"path/filepath"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"gopkg.in/yaml.v3"
)

const (
	chainloopYMLFile  = ".chainloop.yml"
	chainloopYAMLFile = ".chainloop.yaml"
)

// traceWorkflowName is the default workflow name used for trace attestations
// when no override is configured in .chainloop.yml.
const traceWorkflowName = "ai-coding-session"

// ResolveWorkflowName returns the persisted workflow name when non-empty,
// otherwise falls back to the trace default. Centralizes the default so the
// CLI command and the hook handler stay in sync.
func ResolveWorkflowName(persisted string) string {
	if persisted != "" {
		return persisted
	}

	return traceWorkflowName
}

// ChainloopYML represents the relevant fields in .chainloop.yml.
type ChainloopYML struct {
	ProjectName    string `yaml:"projectName"`
	ProjectVersion string `yaml:"projectVersion"`
	// Organization, when set, forces trace attestations (init, add, push)
	// to target that organization instead of the CLI's default.
	Organization string `yaml:"organization,omitempty"`
	// RequireTrace controls whether the pre-push hook blocks the push
	// on attestation failure. nil is treated as false (default-off).
	RequireTrace *bool `yaml:"requireTrace,omitempty"`
	// WorkflowName, when set, overrides the default trace workflow name
	// used when initializing attestations from trace hooks.
	WorkflowName string `yaml:"workflowName,omitempty"`
}

// FindChainloopYML looks for a .chainloop.yml (or .chainloop.yaml) file
// starting from dir and walking up to the git repository root.
// Returns the parsed config if found (with a non-empty projectName), or nil.
func FindChainloopYML(dir string) *ChainloopYML {
	dir = resolveDir(dir)
	repoRoot, err := tracegit.RepoRoot()
	if err != nil {
		return loadChainloopYMLWithProject(dir)
	}
	repoRoot = resolveDir(repoRoot)

	for {
		if cfg := loadChainloopYMLWithProject(dir); cfg != nil {
			return cfg
		}
		if dir == repoRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

// LoadProjectFromYML looks for a .chainloop.yml (or .chainloop.yaml) file
// starting from dir and walking up to the git repository root.
// Returns the projectName if found, or empty string if not found.
func LoadProjectFromYML(dir string) string {
	if cfg := FindChainloopYML(dir); cfg != nil {
		return cfg.ProjectName
	}
	return ""
}

// resolveDir returns an absolute, symlink-resolved path.
// Falls back to the absolute path if symlink resolution fails.
func resolveDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

// SaveProjectToYML writes (or updates) the projectName field in
// .chainloop.yml (or .chainloop.yaml) at the given directory, preserving any other fields.
func SaveProjectToYML(dir, project string) error {
	return updateChainloopYMLField(dir, "projectName", project)
}

// SaveRequireTraceToYML writes (or updates) the requireTrace field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveRequireTraceToYML(dir string, val bool) error {
	return updateChainloopYMLField(dir, "requireTrace", val)
}

// SaveOrganizationToYML writes (or updates) the organization field in
// .chainloop.yml at the given directory, preserving any other fields.
func SaveOrganizationToYML(dir, org string) error {
	return updateChainloopYMLField(dir, "organization", org)
}

// LoadOrganizationFromYML looks for .chainloop.yml starting from dir and returns
// the organization value. Returns empty string when the field is absent.
// Unlike FindChainloopYML, this does not require projectName to be set.
func LoadOrganizationFromYML(dir string) string {
	cfg := findChainloopYMLAny(dir)
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
// Unlike FindChainloopYML, this does not require projectName to be set.
func LoadWorkflowFromYML(dir string) string {
	cfg := findChainloopYMLAny(dir)
	if cfg == nil {
		return ""
	}

	return cfg.WorkflowName
}

// updateChainloopYMLField reads, updates a single field, and writes back
// the .chainloop.yml file, preserving all other fields.
func updateChainloopYMLField(dir, key string, value any) error {
	path := resolveChainloopYMLPath(dir)

	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
		}
	}

	existing[key] = value

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}

	return os.WriteFile(path, out, 0600)
}

// LoadRequireTraceFromYML looks for .chainloop.yml starting from dir
// and returns the requireTrace value. Returns false when the field is
// absent or nil (default-off).
// Unlike FindChainloopYML, this does not require projectName to be set.
func LoadRequireTraceFromYML(dir string) bool {
	cfg := findChainloopYMLAny(dir)
	if cfg == nil || cfg.RequireTrace == nil {
		return false
	}

	return *cfg.RequireTrace
}

// findChainloopYMLAny looks for a .chainloop.yml (or .chainloop.yaml) file
// starting from dir and walking up to the git repository root.
// Unlike FindChainloopYML, it returns any parseable config regardless of
// whether projectName is set.
func findChainloopYMLAny(dir string) *ChainloopYML {
	dir = resolveDir(dir)
	repoRoot, err := tracegit.RepoRoot()
	if err != nil {
		return loadChainloopYMLFromDir(dir)
	}
	repoRoot = resolveDir(repoRoot)

	for {
		if cfg := loadChainloopYMLFromDir(dir); cfg != nil {
			return cfg
		}
		if dir == repoRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil
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

// loadChainloopYMLFromDir loads the ChainloopYML struct from the given directory, or nil if not found.
func loadChainloopYMLFromDir(dir string) *ChainloopYML {
	for _, name := range []string{chainloopYMLFile, chainloopYAMLFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var cfg ChainloopYML
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		return &cfg
	}
	return nil
}

// loadChainloopYMLWithProject is like loadChainloopYMLFromDir but only returns
// a config that has a non-empty projectName. This ensures directory walking
// skips empty or incomplete .chainloop.yml files.
func loadChainloopYMLWithProject(dir string) *ChainloopYML {
	cfg := loadChainloopYMLFromDir(dir)
	if cfg != nil && cfg.ProjectName != "" {
		return cfg
	}
	return nil
}
