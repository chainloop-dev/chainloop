//
// Copyright 2024-2026 The Chainloop Authors.
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

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/google/wire"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

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
	NewCASMappingRepo,
	NewMembershipRepo,
	NewOrgInvitation,
	NewReferrerRepo,
	NewAPITokenRepo,
	NewAttestationStateRepo,
	NewProjectVersionRepo,
	NewProjectsRepo,
	NewGroupRepo,
)

// Data .
type Data struct {
	DB *ent.Client
}

// Load DB schema
// NOTE: this is different than running migrations
// this method is used to load the schema into the DB for TESTING!
func (data *Data) SchemaLoad() error {
	return data.DB.Schema.Create(context.Background())
}

// NewData .
func NewData(c *config.DatabaseConfig, tp trace.TracerProvider, logger log.Logger) (*Data, func(), error) {
	if logger == nil {
		logger = log.NewStdLogger(io.Discard)
	}

	log := log.NewHelper(logger)
	db, err := initSQLDatabase(c, tp, log)
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

	return &Data{DB: db}, cleanup, nil
}

func initSQLDatabase(c *config.DatabaseConfig, tp trace.TracerProvider, log *log.Helper) (*ent.Client, error) {
	if c.Driver != "pgx" {
		return nil, fmt.Errorf("unsupported driver: %s", c.Driver)
	}

	log.Debugf("connecting to db: driver=%s", c.Driver)

	db, err := otelsql.Open(c.Driver, c.Source,
		otelsql.WithTracerProvider(tp),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			DisableErrSkip:       true,
			OmitRows:             true,
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			OmitConnectorConnect: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error opening the connection, driver=%s: %w", c.Driver, err)
	}

	if c.MaxOpenConns > 0 {
		log.Infof("DB: setting max open conns: %d", c.MaxOpenConns)
		db.SetMaxOpenConns(int(c.MaxOpenConns))
	}

	if n := c.MinOpenConns; n > 0 {
		log.Infof("DB: setting min open conns: %v", n)
		// database/sql doesn't have MinOpenConns, but MaxIdleConns serves a similar purpose
		db.SetMaxIdleConns(int(n))
	}

	if t := c.MaxConnIdleTime.AsDuration(); t > 0 {
		log.Infof("DB: setting max conn idle time: %v", t)
		db.SetConnMaxIdleTime(t)
	}

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

func toStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func orgScopedQuery(client *ent.Client, orgID uuid.UUID) *ent.OrganizationQuery {
	return client.Organization.Query().Where(organization.ID(orgID), organization.DeletedAtIsNil())
}

// WithTx initiates a transaction and wraps the DB function
func WithTx(ctx context.Context, client *ent.Client, fn func(tx *ent.Tx) error) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err = fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %w", err, rerr)
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
