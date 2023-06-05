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

package testhelpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	// Requuired for the database waitFor strategy
	_ "github.com/lib/pq"

	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	robotaccount "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/docker/go-connections/nat"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestingUseCases holds all the test data that can be used in the different suites
// NOTE: It connects to a real database
type TestingUseCases struct {
	// Misc
	DB *TestDatabase
	L  log.Logger

	// Use cases
	Membership             *biz.MembershipUseCase
	OCIRepo                *biz.OCIRepositoryUseCase
	Integration            *biz.IntegrationUseCase
	Organization           *biz.OrganizationUseCase
	WorkflowContract       *biz.WorkflowContractUseCase
	Workflow               *biz.WorkflowUseCase
	WorkflowRun            *biz.WorkflowRunUseCase
	User                   *biz.UserUseCase
	RobotAccount           *biz.RobotAccountUseCase
	RegisteredIntegrations sdk.Initialized
}

type newTestingOpts struct {
	credsReaderWriter credentials.ReaderWriter
	integrations      sdk.Initialized
}

type NewTestingUCOpt func(*newTestingOpts)

func WithCredsReaderWriter(rw credentials.ReaderWriter) NewTestingUCOpt {
	return func(tu *newTestingOpts) {
		tu.credsReaderWriter = rw
	}
}

func WithRegisteredIntegration(i sdk.FanOut) NewTestingUCOpt {
	return func(tu *newTestingOpts) {
		if tu.integrations == nil {
			tu.integrations = []sdk.FanOut{i}
		} else {
			tu.integrations = append(tu.integrations, i)
		}
	}
}

func NewTestingUseCases(t *testing.T, opts ...NewTestingUCOpt) *TestingUseCases {
	// default args
	newArgs := &newTestingOpts{credsReaderWriter: creds.NewReaderWriter(t), integrations: make(sdk.Initialized, 0)}

	// Overrides
	for _, opt := range opts {
		opt(newArgs)
	}

	db := NewTestDatabase(t)
	log := log.NewStdLogger(io.Discard)
	testData, _, err := WireTestData(db, t, log, newArgs.credsReaderWriter, &robotaccount.Builder{}, &conf.Auth{
		GeneratedJwsHmacSecret:        "test",
		CasRobotAccountPrivateKeyPath: "./testdata/test-key.ec.pem",
	}, newArgs.integrations)
	assert.NoError(t, err)

	return testData
}

type TestDatabase struct {
	instance testcontainers.Container
}

func NewTestDatabase(t *testing.T) *TestDatabase {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	const port = "5432/tcp"
	dbURL := func(host string, port nat.Port) string {
		return fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())
	}

	req := testcontainers.ContainerRequest{
		Image:        "postgres:14-alpine",
		ExposedPorts: []string{port},
		AutoRemove:   true,
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "controlplane_test",
		},
		WaitingFor: wait.ForSQL(nat.Port(port), "postgres", dbURL).WithStartupTimeout(time.Second * 5),
	}

	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	return &TestDatabase{
		instance: postgres,
	}
}

func (db *TestDatabase) Port(t *testing.T) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, err := db.instance.MappedPort(ctx, "5432")
	require.NoError(t, err)
	return p.Int()
}

func (db *TestDatabase) ConnectionString(t *testing.T) string {
	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres", db.Port(t))
}

func newConfData(db *TestDatabase, t *testing.T) *conf.Data {
	return &conf.Data{Database: &conf.Data_Database{Driver: "pgx", Source: db.ConnectionString(t)}}
}

func (db *TestDatabase) Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require.NoError(t, db.instance.Terminate(ctx))
}

// We use an env variable because testing flags will require us to add them to each testing package
func IntegrationTestsEnabled() bool {
	return os.Getenv("SKIP_INTEGRATION") != "true"
}
