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

package data

import (
	"context"
	"fmt"
	"io"
	"time"

	"database/sql"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/organization"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/google/wire"

	// Load PGX driver
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewWorkflowRepo,
	NewUserRepo,
	NewRobotAccountRepo,
	NewWorkflowRunRepo,
	NewOrganizationRepo,
	NewWorkflowContractRepo,
	NewCASBackendRepo,
	NewOrgMetricsRepo,
	NewIntegrationRepo,
	NewIntegrationAttachmentRepo,
	NewMembershipRepo,
)

// Data .
type Data struct {
	db *ent.Client
}

// Load DB schema
// NOTE: this is different than running migrations
// this method is used to load the schema into the DB for TESTING!
func (data *Data) SchemaLoad() error {
	return data.db.Schema.Create(context.Background())
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	log := log.NewHelper(logger)
	db, err := initSQLDatabase(c.Database, log)
	if err != nil {
		log.Errorf("error initialing the DB : %v", err)
		return nil, nil, fmt.Errorf("failed to initialized db: %w", err)
	}

	cleanup := func() {
		log.Info("closing the data resources")
		if err := db.Close(); err != nil {
			log.Error(err)
		}
	}

	return &Data{db: db}, cleanup, nil
}

func initSQLDatabase(c *conf.Data_Database, log *log.Helper) (*ent.Client, error) {
	log.Debugf("connecting to db: driver=%s", c.Driver)
	db, err := sql.Open(
		c.Driver,
		c.Source,
	)
	if err != nil {
		return nil, fmt.Errorf("error opening the connection, driver=%s:  %w", c.Driver, err)
	}

	// Create an ent.Driver from `db`.
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))

	// NOTE: We do not run migrations automatically anymore
	// Instead we leverage atlas cli to run migrations
	return client, nil
}

func toTimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

func orgScopedQuery(client *ent.Client, orgID uuid.UUID) *ent.OrganizationQuery {
	return client.Organization.Query().Where(organization.ID(orgID))
}
