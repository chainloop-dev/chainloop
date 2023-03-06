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

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	awssecrets "github.com/chainloop-dev/chainloop/internal/credentials/aws"
	"github.com/chainloop-dev/chainloop/internal/credentials/vault"
	"github.com/getsentry/sentry-go"

	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

var (
	// Name is the name of the compiled software.
	Name string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

// Version is the version of the compiled software.
// go build ldflags "-X main.Version=x.y.z"
var Version = servicelogger.Dev

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.HTTPMetricsServer) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			ms,
		),
	)
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			// -conf [config directory or path to file]
			file.NewSource(flagconf),
			// Load environments variables prefixed with CAS_
			// NOTE: They get resolved withouth the prefix, i.e CAS_DB_HOST -> DB_HOST
			env.NewSource("CAS_"),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	logger, err := servicelogger.InitZapLogger(Version)
	if err != nil {
		panic(err)
	}

	flush, err := initSentry(&bc, logger)
	defer flush()
	if err != nil {
		panic(err)
	}

	credentialsReader, err := newCredentialsReader(bc.GetCredentialsService(), logger)
	if err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, bc.Auth, credentialsReader, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func newCredentialsReader(conf *conf.Credentials, l log.Logger) (credentials.Reader, error) {
	awsc, vaultc := conf.GetAwsSecretManager(), conf.GetVault()
	if awsc == nil && vaultc == nil {
		return nil, errors.New("no credentials manager configuration found")
	} else if awsc != nil && vaultc != nil {
		return nil, errors.New("only one credentials manager can be configured")
	}

	if c := conf.GetAwsSecretManager(); c != nil {
		return newAWSCredentialsManager(c, l)
	}

	return newVaultCredentialsManager(conf.GetVault(), l)
}

func newAWSCredentialsManager(conf *conf.Credentials_AWSSecretManager, l log.Logger) (*awssecrets.Manager, error) {
	if conf == nil {
		return nil, errors.New("incompleted configuration for AWS secret manager")
	}

	opts := &awssecrets.NewManagerOpts{
		Region:    conf.Region,
		AccessKey: conf.GetCreds().GetAccessKey(), SecretKey: conf.GetCreds().GetSecretKey(),
		Logger: l,
	}

	m, err := awssecrets.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring the secrets manager: %w", err)
	}

	_ = l.Log(log.LevelInfo, "msg", "secrets manager configured", "backend", "AWS secret manager")

	return m, nil
}

func newVaultCredentialsManager(conf *conf.Credentials_Vault, l log.Logger) (*vault.Manager, error) {
	if conf == nil {
		return nil, errors.New("incompleted configuration for vault credentials manager")
	}

	opts := &vault.NewManagerOpts{
		AuthToken: conf.Token, Address: conf.Address,
		MountPath: conf.MountPath, Logger: l,
	}

	m, err := vault.NewManager(opts)
	if err != nil {
		return nil, fmt.Errorf("configuring vault: %w", err)
	}

	_ = l.Log(log.LevelInfo, "msg", "secrets manager configured", "backend", "Vault")

	return m, nil
}

func initSentry(c *conf.Bootstrap, logger log.Logger) (cleanupFunc func(), err error) {
	cleanupFunc = func() {
		sentry.Flush(2 * time.Second)
	}

	if c.Observability == nil || c.Observability.Sentry == nil {
		return
	}

	sentryOpts := c.Observability.Sentry
	err = sentry.Init(sentry.ClientOptions{
		Dsn:              sentryOpts.Dsn,
		Environment:      sentryOpts.Environment,
		Release:          Version,
		AttachStacktrace: true,
	})

	if err == nil {
		_ = logger.Log(log.LevelInfo, "msg", "Sentry initialized", "environment", sentryOpts.Environment, "release", Version)
	}

	return
}
