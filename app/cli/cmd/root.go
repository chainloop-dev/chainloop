//
// Copyright 2023 The Chainloop Authors.
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
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/adrg/xdg"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry/posthog"
	"github.com/chainloop-dev/chainloop/internal/grpcconn"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	flagCfgFile      string
	flagInsecure     bool
	flagDebug        bool
	flagOutputFormat string
	actionOpts       *action.ActionsOpts
	logger           zerolog.Logger
	defaultCPAPI     = "api.cp.chainloop.dev:443"
	defaultCASAPI    = "api.cas.chainloop.dev:443"
	apiToken         string
)

const (
	useAPIToken = "withAPITokenAuth"
	appName     = "chainloop"
	//nolint:gosec
	tokenEnvVarName = "CHAINLOOP_TOKEN"
	userAudience    = "user-auth.chainloop"
	//nolint:gosec
	apiTokenAudience = "api-token-auth.chainloop"
	// Follow the convention stated on https://consoledonottrack.com/
	doNotTrackEnv = "DO_NOT_TRACK"
)

var telemetryWg sync.WaitGroup

type parsedToken struct {
	id        string
	orgID     string
	tokenType string
}

func NewRootCmd(l zerolog.Logger) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           appName,
		Short:         "Chainloop Command Line Interface",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug().Str("path", viper.ConfigFileUsed()).Msg("using config file")

			var err error
			logger, err = initLogger(l)
			if err != nil {
				return err
			}

			if flagInsecure {
				logger.Warn().Msg("API contacted in insecure mode")
			}

			apiToken, err := loadControlplaneAuthToken(cmd)
			if err != nil {
				return fmt.Errorf("loading controlplane auth token: %w", err)
			}

			conn, err := grpcconn.New(viper.GetString(confOptions.controlplaneAPI.viperKey), apiToken, flagInsecure)
			if err != nil {
				return err
			}

			actionOpts = newActionOpts(logger, conn)

			if !isTelemetryDisabled() {
				logger.Debug().Msg("Telemetry enabled, to disable it use DO_NOT_TRACK=1")

				telemetryWg.Add(1)
				go func() {
					defer telemetryWg.Done()

					// For telemetry reasons we parse the token to know the type of token is being used when executing the CLI
					// Once we have the token type we can send it to the telemetry service by injecting it on the context
					token, err := parseToken(apiToken)
					if err != nil {
						logger.Debug().Err(err).Msg("parsing token for telemetry")
						return
					}

					err = recordCommand(cmd, token)
					if err != nil {
						logger.Debug().Err(err).Msg("sending command to telemetry")
					}
				}()
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return cleanup(actionOpts.CPConnection)
		},
	}

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	rootCmd.PersistentFlags().StringVarP(&flagCfgFile, "config", "c", "", "Path to an existing config file (default is $HOME/.config/chainloop/config.toml)")

	rootCmd.PersistentFlags().String(confOptions.controlplaneAPI.flagName, defaultCPAPI, "URL for the Control Plane API")
	err := viper.BindPFlag(confOptions.controlplaneAPI.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.controlplaneAPI.flagName))
	cobra.CheckErr(err)

	rootCmd.PersistentFlags().String(confOptions.CASAPI.flagName, defaultCASAPI, "URL for the Artifacts Content Addressable Storage (CAS)")
	err = viper.BindPFlag(confOptions.CASAPI.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.CASAPI.flagName))
	cobra.CheckErr(err)

	rootCmd.PersistentFlags().BoolVarP(&flagInsecure, "insecure", "i", false, "Skip TLS transport during connection to the control plane")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug/verbose logging mode")
	rootCmd.PersistentFlags().StringVarP(&flagOutputFormat, "output", "o", "table", "Output format, valid options are json and table")

	// Override the oauth authentication requirement for the CLI by providing an API token
	rootCmd.PersistentFlags().StringVarP(&apiToken, "token", "t", "", fmt.Sprintf("API token. NOTE: Alternatively use the env variable %s", tokenEnvVarName))
	// We do not use viper in this case because we do not want this token to be saved in the config file
	// Instead we load the env variable manually
	if apiToken == "" {
		apiToken = os.Getenv(tokenEnvVarName)
	}

	rootCmd.AddCommand(newWorkflowCmd(), newAuthCmd(), NewVersionCmd(),
		newAttestationCmd(), newArtifactCmd(), newConfigCmd(),
		newIntegrationCmd(), newOrganizationCmd(), newCASBackendCmd(),
		newReferrerDiscoverCmd(),
	)

	return rootCmd
}

func init() {
	cobra.OnInitialize(initConfigFile)
	// Using the cobra.OnFinalize because the hooks don't work on error
	cobra.OnFinalize(func() {
		// In some cases the command is faster than the telemetry, in that case we wait
		telemetryWg.Wait()
	})
}

// isTelemetryDisabled checks if the telemetry is disabled by the user or if we are running a development version
func isTelemetryDisabled() bool {
	return os.Getenv(doNotTrackEnv) == "1" || os.Getenv(doNotTrackEnv) == "true" || Version == devVersion
}

func initLogger(logger zerolog.Logger) (zerolog.Logger, error) {
	lvl := zerolog.InfoLevel
	if flagDebug {
		lvl = zerolog.DebugLevel
	}

	return logger.Level(lvl), nil
}

func initConfigFile() {
	// An existing config file was passed as a flag and we use it as is
	if flagCfgFile != "" {
		viper.SetConfigFile(flagCfgFile)
		cobra.CheckErr(viper.ReadInConfig())
		return
	}

	// If no config file was passed as a flag we use the default one
	configPath := filepath.Join(xdg.ConfigHome, appName)
	// Create the file if it does not exist
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(configPath, os.ModePerm)
		if err != nil {
			panic(fmt.Errorf("creating config file %s: %w", configPath, err))
		}
	}

	viper.AddConfigPath(configPath)
	viper.SetConfigType("toml")

	// Development uses a different version of the config file
	configName := "config"
	if Version == devVersion {
		configName = "config.devel"
	}

	viper.SetConfigName(configName)

	// Write the file only if it does not exist yet
	err := viper.SafeWriteConfig()

	// Capture the error if it's not that the file exists
	wantErr := viper.ConfigFileAlreadyExistsError("")
	if !errors.As(err, &wantErr) {
		cobra.CheckErr(err)
	}

	cobra.CheckErr(viper.ReadInConfig())
}

func newActionOpts(logger zerolog.Logger, conn *grpc.ClientConn) *action.ActionsOpts {
	return &action.ActionsOpts{CPConnection: conn, Logger: logger, UseAttestationRemoteState: useAttestationRemoteState}
}

func cleanup(conn *grpc.ClientConn) error {
	if conn != nil {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Load the controlplane based on the following order:
// 1. If the CMD uses an API token flag/env variable we override it
// 2. If the CMD uses a config file we load it from there
func loadControlplaneAuthToken(cmd *cobra.Command) (string, error) {
	// If the CMD uses an API token instead of the regular OIDC auth token we override it
	// TODO: the attestation CLI should get split from this one
	if _, ok := cmd.Annotations[useAPIToken]; ok {
		if attAPIToken == "" {
			return "", newGracefulError(ErrAttestationTokenRequired)
		}

		return attAPIToken, nil
	}

	// override if token is passed as a flag/env variable
	if apiToken != "" {
		logger.Info().Msg("API token provided to the command line")
		return apiToken, nil
	}

	// loaded from config file, previously stored via "auth login"
	return viper.GetString(confOptions.authToken.viperKey), nil
}

// parseToken the token and return the type of token. At the moment in Chainloop we have 3 types of tokens:
// 1. User account token
// 2. API token
// Each one of them have an associated audience claim that we use to identify the type of token. If the token is not
// present, nor we cannot match it with one of the expected audience, return nil.
func parseToken(token string) (*parsedToken, error) {
	if token == "" {
		return nil, nil
	}

	// Create a parser without claims validation
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	// Parse the token without verification
	t, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	// Extract generic claims otherwise, we would have to parse
	// the token again to get the claims for each type
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil
	}

	// Get the audience claim
	val, ok := claims["aud"]
	if !ok || val == nil {
		return nil, nil
	}

	// Ensure audience is an array of interfaces
	// Chainloop only has one audience per token
	aud, ok := val.([]interface{})
	if !ok || len(aud) == 0 {
		return nil, nil
	}

	// Initialize parsedToken
	pToken := &parsedToken{}

	// Determine the type of token based on the audience.
	switch aud[0].(string) {
	case apiTokenAudience:
		pToken.tokenType = "api-token"
		if tokenID, ok := claims["jti"].(string); ok {
			pToken.id = tokenID
		}
		if orgID, ok := claims["org_id"].(string); ok {
			pToken.orgID = orgID
		}
	case userAudience:
		pToken.tokenType = "user"
		if userID, ok := claims["user_id"].(string); ok {
			pToken.id = userID
		}
	default:
		return nil, nil
	}

	return pToken, nil
}

var (
	// Posthog API key and endpoint are not sensitive information it represents Chainloop's Posthog instance.
	// It can be overridden by the user if they want to use their own instance of Posthog or deactivated by setting
	// DO_NOT_TRACK=1 more information that can be found at: https://github.com/chainloop-dev/chainloop/blob/main/docs/docs/reference/operator/cli-telemetry.mdx
	// nolint:gosec
	posthogAPIKey   = "phc_TWWW19kEiD6sEejlHKWcICQ5Vc06vZUTYia8WdPB0A0"
	posthogEndpoint = "https://crb.chainloop.dev"
)

// recordCommand sends the command to the telemetry service
func recordCommand(executedCmd *cobra.Command, authInfo *parsedToken) error {
	telemetryClient, err := posthog.NewClient(posthogAPIKey, posthogEndpoint)
	if err != nil {
		logger.Debug().Err(err).Msgf("creating telemetry client: %v", err)
		return nil
	}

	cmdTracker := telemetry.NewCommandTracker(telemetryClient)
	tags := telemetry.Tags{
		"cli_version":      Version,
		"cp_url_hash":      hashControlPlaneURL(),
		"chainloop_source": "cli",
	}

	// It tries to extract the token from the context and add it to the tags. If it fails, it will ignore it.
	if authInfo != nil {
		tags["token_type"] = authInfo.tokenType
		tags["user_id"] = authInfo.id
		tags["org_id"] = authInfo.orgID
	}

	if err = cmdTracker.Track(executedCmd.Context(), extractCmdLineFromCommand(executedCmd), tags); err != nil {
		return fmt.Errorf("sending event: %w", err)
	}

	return nil
}

// extractCmdLineFromCommand returns the full command hierarchy as a string from a cobra.Command
func extractCmdLineFromCommand(cmd *cobra.Command) string {
	var cmdHierarchy []string
	currentCmd := cmd
	// While the current command is not the root command, keep iteration.
	// This is done to get the full hierarchy of the command and remove the root command from the hierarchy.
	for currentCmd.Use != "chainloop" {
		cmdHierarchy = append([]string{currentCmd.Use}, cmdHierarchy...)
		currentCmd = currentCmd.Parent()
	}

	cmdLine := strings.Join(cmdHierarchy, " ")
	return cmdLine
}

// hashControlPlaneURL returns a hash of the control plane URL
func hashControlPlaneURL() string {
	url := viper.GetString("control-plane.API")

	return fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
}
