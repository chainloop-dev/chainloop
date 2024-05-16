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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/chainloop-dev/chainloop/app/cli/internal/action"
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
	useWorkflowRobotAccount = "withWorkflowRobotAccount"
	appName                 = "chainloop"
	//nolint:gosec
	tokenEnvVarName      = "CHAINLOOP_TOKEN"
	robotAccountAudience = "attestations.chainloop"
	userAudience         = "user-auth.chainloop"
	//nolint:gosec
	apiTokenAudience = "api-token-auth.chainloop"
)

type AuthenticationToken struct{}
type ParsedToken struct {
	ID   string
	Type string
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

			token, err := parseToken(apiToken)
			if err != nil {
				logger.Debug().Err(err).Msg("parsing token for telemetry")
			}

			// Inject the authentication token into the command context
			cmd.SetContext(context.WithValue(cmd.Context(), AuthenticationToken{}, token))

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
// 1. If the CMD uses a robot account instead of the regular auth token we override it
// 2. If the CMD uses an API token flag/env variable we override it
// 3. If the CMD uses a config file we load it from there
func loadControlplaneAuthToken(cmd *cobra.Command) (string, error) {
	// If the CMD uses a robot account instead of the regular auth token we override it
	// TODO: the attestation CLI should get split from this one
	if _, ok := cmd.Annotations[useWorkflowRobotAccount]; ok {
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

// parseToken the token and return the type of token
func parseToken(token string) (*ParsedToken, error) {
	if token == "" {
		return &ParsedToken{}, nil
	}

	// Create a parser without claims validation
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	// Parse the token without verification
	parsedToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	// Extract claims
	claims := parsedToken.Claims.(jwt.MapClaims)

	// Get the audience claim
	if val, ok := claims["aud"]; ok && val != nil {
		// Chainloop tokens have only one audience in an array
		aud, ok := val.([]interface{})
		if !ok {
			return &ParsedToken{}, nil
		}
		if len(aud) == 0 {
			return &ParsedToken{}, nil
		}

		switch aud[0].(string) {
		case apiTokenAudience:
			return &ParsedToken{Type: "api-token"}, nil
		case userAudience:
			userID := claims["user_id"].(string)
			return &ParsedToken{Type: "user", ID: userID}, nil
		case robotAccountAudience:
			return &ParsedToken{Type: "robot-account"}, nil
		default:
			return &ParsedToken{}, nil
		}
	}

	return &ParsedToken{}, nil
}
