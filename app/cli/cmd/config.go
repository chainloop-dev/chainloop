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

	maxParentDirTraversals = 3
)

var (
	errDotChainloopConfigNotFound = fmt.Errorf("attestation config file %s not found", dotChainloopConfigFilename)
	dotChainloopConfigExtension   = []string{".yaml", ".yml"}
)

// loadDotChainloopConfigWithParentTraversal attempts to load the attestation configuration file from the current directory
// and its parent directories up to a maximum of maxParentDirTraversals. If the file is found, it
// is decoded and returned. If the file is not found, an error is returned.
func loadDotChainloopConfigWithParentTraversal() (*DotChainloopConfig, string, error) {
	currentDir := "."
	for i := 0; i < maxParentDirTraversals; i++ {
		for _, ext := range dotChainloopConfigExtension {
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
		currentDir = filepath.Join(currentDir, "..")
	}

	return nil, "", errDotChainloopConfigNotFound
}

type DotChainloopConfig struct {
	Version string `yaml:"version"`
}
