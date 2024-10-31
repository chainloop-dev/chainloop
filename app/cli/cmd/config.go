//
// Copyright 2024 The Chainloop Authors.
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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Map of all the possible configuration options that we expect viper to handle
var confOptions = struct {
	authToken, controlplaneAPI, CASAPI, controlplaneCA, CASCA, insecure *confOpt
}{
	insecure: &confOpt{
		viperKey: "api-insecure",
	},
	authToken: &confOpt{
		viperKey: "auth.token",
	},
	controlplaneAPI: &confOpt{
		viperKey: "control-plane.API",
		flagName: "control-plane",
	},
	controlplaneCA: &confOpt{
		viperKey: "control-plane.api-ca",
		flagName: "control-plane-ca",
	},
	CASAPI: &confOpt{
		viperKey: "artifact-cas.API",
		flagName: "artifact-cas",
	},
	CASCA: &confOpt{
		viperKey: "artifact-cas.api-ca",
		flagName: "artifact-cas-ca",
	},
}

type confOpt struct {
	// The key used to store the value in viper
	viperKey string
	// The flag name used during viper bind/override
	flagName string
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure this client",
	}

	cmd.AddCommand(newConfigSaveCmd(), newConfigViewCmd(), newConfigResetCmd())
	return cmd
}

// Configuration file used in repositories ot modify the attestation process
const (
	dotChainloopConfigFilename = ".chainloop"
)

var (
	errDotChainloopConfigNotFound = fmt.Errorf("attestation config file %s.[yaml|yml] not found", dotChainloopConfigFilename)
)

// LoadDotChainloopConfig loads the chainloop config file from the current directory all the way up to the root directory
// or until a .git directory is found
// It supports both .yaml and .yml extensions
func loadDotChainloopConfigWithParentTraversal() (*DotChainloopConfig, string, error) {
	searchPaths := getConfigSearchPaths()
	logger.Debug().Msgf("searching %s.[yaml|yml] file in %q", dotChainloopConfigFilename, searchPaths)

	for _, currentDir := range searchPaths {
		for _, ext := range []string{".yaml", ".yml"} {
			configPath := filepath.Join(currentDir, fmt.Sprintf("%s%s", dotChainloopConfigFilename, ext))
			// Check if the file exists
			if _, err := os.Stat(configPath); !os.IsNotExist(err) {
				// Load from the YAML file
				file, err := os.Open(configPath)
				if err != nil {
					return nil, "", fmt.Errorf("opening attestation config file: %w", err)
				}
				defer file.Close()

				cfg := &DotChainloopConfig{}
				decoder := yaml.NewDecoder(file)
				if err := decoder.Decode(cfg); err != nil {
					return nil, "", fmt.Errorf("decoding attestation config file: %w", err)
				}

				return cfg, configPath, nil
			}
		}
	}

	return nil, "", errDotChainloopConfigNotFound
}

// getConfigSearchPaths returns a slice of directory paths to search for the attestation configuration file.
// It starts from the current working directory and traverses up the directory tree up all the way to the root
// or until it finds a .git directory. The search paths are returned in order, with the current directory first.
// Based out of golangci-lint traverse mechanism
func getConfigSearchPaths() []string {
	absPath, err := filepath.Abs(".")
	if err != nil {
		absPath = filepath.Clean(".")
	}

	var currentDir string
	if isDir(absPath) {
		currentDir = absPath
	} else {
		currentDir = filepath.Dir(absPath)
	}

	// find all dirs from it up to the root
	searchPaths := []string{"./"}

	for {
		searchPaths = append(searchPaths, currentDir)

		parent := filepath.Dir(currentDir)
		if currentDir == parent || parent == "" {
			break
		}

		// We also terminate if there is a .git directory
		if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
			break
		}

		currentDir = parent
	}

	return searchPaths
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

type DotChainloopConfig struct {
	ProjectVersion string `yaml:"projectVersion"`
}
