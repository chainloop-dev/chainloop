//
// Copyright 2024-2025 The Chainloop Authors.
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
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry"
	"github.com/chainloop-dev/chainloop/app/cli/internal/telemetry/posthog"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/grpcconn"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	flagCfgFile      string
	flagDebug        bool
	flagOutputFormat string
	actionOpts       *action.ActionsOpts
	logger           zerolog.Logger
	defaultCPAPI     = "api.cp.chainloop.dev:443"
	defaultCASAPI    = "api.cas.chainloop.dev:443"
	apiToken         string
	flagYes          bool
)

const (
	// preference to use an API token if available
	useAPIToken = "withAPITokenAuth"
	// Ask for confirmation when user token is used and API token is preferred
	confirmWhenUserToken = "confirmWhenUserToken"
	appName              = "chainloop"
	//nolint:gosec
	tokenEnvVarName = "CHAINLOOP_TOKEN"
	userAudience    = "user-auth.chainloop"
	//nolint:gosec
	apiTokenAudience = "api-token-auth.chainloop"
	// Follow the convention stated on https://consoledonottrack.com/
	doNotTrackEnv = "DO_NOT_TRACK"

	trueString = "true"
)

var telemetryWg sync.WaitGroup

type parsedToken struct {
	id        string
	orgID     string
	tokenType string
}

// Environment variable prefix for vipers
const envPrefix = "CHAINLOOP"

func Execute(l zerolog.Logger) error {
	rootCmd := NewRootCmd(l)
	if err := rootCmd.Execute(); err != nil {
		// The local file is pointing to the wrong organization, we remove it
		if v1.IsUserNotMemberOfOrgErrorNotInOrg(err) {
			if err := setLocalOrganization(""); err != nil {
				logger.Debug().Err(err).Msg("failed to remove organization from config")
			}
		}

		return err
	}

	return nil
}

func NewRootCmd(l zerolog.Logger) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           appName,
		Short:         "Chainloop Command Line Interface",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			logger, err = initLogger(l)
			if err != nil {
				return err
			}

			logger.Debug().Str("path", viper.ConfigFileUsed()).Msg("using config file")

			if apiInsecure() {
				logger.Warn().Msg("API contacted in insecure mode")
			}

			token, isUserToken, err := loadControlplaneAuthToken(cmd)
			if err != nil {
				return err
			}

			var opts = []grpcconn.Option{
				grpcconn.WithInsecure(apiInsecure()),
			}

			if caFilePath := viper.GetString(confOptions.controlplaneCA.viperKey); caFilePath != "" {
				opts = append(opts, grpcconn.WithCAFile(caFilePath))
			}

			controlplaneURL := viper.GetString(confOptions.controlplaneAPI.viperKey)

			// If no organization is set in local configuration, we load it from server and save it
			orgName := viper.GetString(confOptions.organization.viperKey)
			if orgName == "" {
				conn, err := grpcconn.New(controlplaneURL, token, opts...)
				if err != nil {
					return err
				}

				currentContext, err := action.NewConfigCurrentContext(newActionOpts(logger, conn, token)).Run()
				if err == nil && currentContext.CurrentMembership != nil {
					if err := setLocalOrganization(currentContext.CurrentMembership.Org.Name); err != nil {
						return fmt.Errorf("writing config file: %w", err)
					}
				}
			}

			// reload the connection now that we have the org name
			orgName = viper.GetString(confOptions.organization.viperKey)
			if orgName != "" {
				opts = append(opts, grpcconn.WithOrgName(orgName))
			}

			// Warn users when the session is interactive, and the operation is supposed to use an API token instead
			if shouldAskForConfirmation(cmd) && isUserToken && !flagYes {
				if !confirmationPrompt(fmt.Sprintf("This command is will run against the organization %q", orgName)) {
					return errors.New("command canceled by user")
				}
			}

			conn, err := grpcconn.New(controlplaneURL, token, opts...)
			if err != nil {
				return err
			}
			actionOpts = newActionOpts(logger, conn, token)

			if !isTelemetryDisabled() {
				logger.Debug().Msg("Telemetry enabled, to disable it use DO_NOT_TRACK=1")

				telemetryWg.Add(1)
				go func() {
					defer telemetryWg.Done()

					// Create a context that times out after 1 seconds, this is because posthog has a 10 seconds hardcoded timeout
					// and we want to finish earlier than that
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					done := make(chan struct{})

					go func() {
						// For telemetry reasons we parse the token to know the type of token is being used when executing the CLI
						// Once we have the token type we can send it to the telemetry service by injecting it on the context
						token, err := parseToken(token)
						if err != nil {
							logger.Debug().Err(err).Msg("parsing token for telemetry")
							return
						}

						err = recordCommand(cmd, token)
						if err != nil {
							logger.Debug().Err(err).Msg("sending command to telemetry")
						}
						close(done)
					}()

					select {
					case <-done:
						// The parsing and recording finished successfully within the timeout
					case <-ctx.Done():
						// The operation took more than timeout
					}
				}()
			}

			return nil
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			return cleanup(actionOpts.CPConnection)
		},
	}

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	rootCmd.PersistentFlags().StringVarP(&flagCfgFile, "config", "c", "", "Path to an existing config file (default is $HOME/.config/chainloop/config.toml)")

	rootCmd.PersistentFlags().String(confOptions.controlplaneAPI.flagName, defaultCPAPI, fmt.Sprintf("URL for the Control Plane API ($%s)", calculateEnvVarName(confOptions.controlplaneAPI.viperKey)))
	cobra.CheckErr(viper.BindPFlag(confOptions.controlplaneAPI.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.controlplaneAPI.flagName)))
	cobra.CheckErr(viper.BindEnv(confOptions.controlplaneAPI.viperKey, calculateEnvVarName(confOptions.controlplaneAPI.viperKey)))

	// Custom CAs for the control plane
	rootCmd.PersistentFlags().String(confOptions.controlplaneCA.flagName, "", fmt.Sprintf("CUSTOM CA file for the Control Plane API (optional) ($%s)", calculateEnvVarName(confOptions.controlplaneCA.viperKey)))
	cobra.CheckErr(viper.BindPFlag(confOptions.controlplaneCA.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.controlplaneCA.flagName)))
	cobra.CheckErr(viper.BindEnv(confOptions.controlplaneCA.viperKey, calculateEnvVarName(confOptions.controlplaneCA.viperKey)))

	rootCmd.PersistentFlags().String(confOptions.CASAPI.flagName, defaultCASAPI, fmt.Sprintf("URL for the Artifacts Content Addressable Storage API ($%s)", calculateEnvVarName(confOptions.CASAPI.viperKey)))
	cobra.CheckErr(viper.BindPFlag(confOptions.CASAPI.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.CASAPI.flagName)))
	cobra.CheckErr(viper.BindEnv(confOptions.CASAPI.viperKey, calculateEnvVarName(confOptions.CASAPI.viperKey)))

	// Custom CAs for the CAS
	rootCmd.PersistentFlags().String(confOptions.CASCA.flagName, "", fmt.Sprintf("CUSTOM CA file for the Artifacts CAS API (optional) ($%s)", calculateEnvVarName(confOptions.CASCA.viperKey)))
	cobra.CheckErr(viper.BindPFlag(confOptions.CASCA.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.CASCA.flagName)))
	cobra.CheckErr(viper.BindEnv(confOptions.CASCA.viperKey, calculateEnvVarName(confOptions.CASCA.viperKey)))

	rootCmd.PersistentFlags().BoolP("insecure", "i", false, fmt.Sprintf("Skip TLS transport during connection to the control plane ($%s)", calculateEnvVarName(confOptions.insecure.viperKey)))
	cobra.CheckErr(viper.BindPFlag(confOptions.insecure.viperKey, rootCmd.PersistentFlags().Lookup("insecure")))
	cobra.CheckErr(viper.BindEnv(confOptions.insecure.viperKey, calculateEnvVarName(confOptions.insecure.viperKey)))

	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug/verbose logging mode")
	rootCmd.PersistentFlags().StringVarP(&flagOutputFormat, "output", "o", "table", "Output format, valid options are json and table")

	// Override the oauth authentication requirement for the CLI by providing an API token
	rootCmd.PersistentFlags().StringVarP(&apiToken, "token", "t", "", fmt.Sprintf("API token. NOTE: Alternatively use the env variable %s", tokenEnvVarName))

	rootCmd.PersistentFlags().StringP(confOptions.organization.flagName, "n", "", "organization name")
	cobra.CheckErr(viper.BindPFlag(confOptions.organization.viperKey, rootCmd.PersistentFlags().Lookup(confOptions.organization.flagName)))

	// Do not ask for confirmation
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")

	rootCmd.AddCommand(newWorkflowCmd(), newAuthCmd(), NewVersionCmd(),
		newAttestationCmd(), newArtifactCmd(), newConfigCmd(),
		newIntegrationCmd(), newOrganizationCmd(), newCASBackendCmd(),
		newReferrerDiscoverCmd(),
	)

	return rootCmd
}

// this could have been done using automatic + prefix but we want to have control and know the values
//
//	viper.AutomaticEnv()
//	viper.SetEnvPrefix(envPrefix)
//	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
func calculateEnvVarName(key string) string {
	// replace - with _ and . with _
	s := strings.ReplaceAll(key, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return fmt.Sprintf("%s_%s", envPrefix, strings.ToUpper(s))
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
	return os.Getenv(doNotTrackEnv) == "1" || os.Getenv(doNotTrackEnv) == trueString || Version == devVersion
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

func newActionOpts(logger zerolog.Logger, conn *grpc.ClientConn, token string) *action.ActionsOpts {
	return &action.ActionsOpts{CPConnection: conn, Logger: logger, AuthTokenRaw: token}
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
// 1. If the command requires an API token, we load it and fail otherwise
// 2. If the command does not require an API token, we
// 2.1 Return the token explicitly provided via the flag
// 2.2 Load the token from the environment variable and from the auth login config file
// 2.3 if they both exist, we default to the user token
// 2.4 otherwise to the one that's set
func loadControlplaneAuthToken(cmd *cobra.Command) (string, bool, error) {
	// Load the APIToken from the env variable
	apiTokenFromVar := os.Getenv(tokenEnvVarName)

	// Load the user token from the config file
	userToken := viper.GetString(confOptions.authToken.viperKey)

	apiTokenFromFlagOrVar := apiToken
	if apiTokenFromFlagOrVar == "" {
		apiTokenFromFlagOrVar = apiTokenFromVar
	}

	// Prefer to use the API token if the command can use it, and it's provided (i.e. attestations)
	if isAPITokenPreferred(cmd) && apiTokenFromFlagOrVar != "" {
		return apiTokenFromFlagOrVar, false, nil
	}

	// Now we check explicitly provided API token via the flag
	if apiToken != "" {
		logger.Info().Msg("API token provided to the command line")
		return apiToken, false, nil
	}

	// If both the user authentication and the API token en var are set, we default to user authentication
	if userToken != "" && apiTokenFromVar != "" {
		logger.Warn().Msgf("Both user credentials and $%s set. Ignoring $%s.", tokenEnvVarName, tokenEnvVarName)
		return userToken, true, nil
	} else if apiTokenFromVar != "" {
		return apiTokenFromVar, false, nil
	}

	return userToken, true, nil
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

func apiInsecure() bool {
	return viper.GetBool(confOptions.insecure.viperKey)
}

// setLocalOrganization updates the local organization configuration
func setLocalOrganization(orgName string) error {
	viper.Set(confOptions.organization.viperKey, orgName)
	return viper.WriteConfig()
}

func shouldAskForConfirmation(cmd *cobra.Command) bool {
	return isAPITokenPreferred(cmd) && cmd.Annotations[confirmWhenUserToken] == trueString
}

func isAPITokenPreferred(cmd *cobra.Command) bool {
	return cmd.Annotations[useAPIToken] == trueString
}
