//
// Copyright 2026 The Chainloop Authors.
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

package ent

import (
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

type driverWithDB struct {
	dialect.Driver
	db *sql.DB
}

func (d *driverWithDB) DB() *sql.DB { return d.db }

func TestPingWithWrappedDriver(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost:1/unused")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	client := NewClient(Driver(&driverWithDB{db: db}))
	require.EqualError(t, client.Ping(), "sql: database is closed")
}
