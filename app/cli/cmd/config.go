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
	"github.com/spf13/cobra"
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
