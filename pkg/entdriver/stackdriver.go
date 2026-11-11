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

// Package entdriver provides ent driver wrappers.
package entdriver

import (
	"context"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/pkg/errorsx"
)

// StackDriver is an ent driver that records the stack of the caller on each
// database error. The stack holds the repository method that ran the query,
// so Sentry groups a database failure by the query that failed.
// Do not run ent's schema API (Schema.Create, Diff, WriteTo) through it: that
// code casts the result rows to *sql.Rows, which stackRows is not.
type StackDriver struct {
	// Driver is the ent driver that runs the queries.
	*entsql.Driver
}

// NewStackDriver wraps driver so database errors record the caller's stack.
func NewStackDriver(driver *entsql.Driver) *StackDriver {
	return &StackDriver{Driver: driver}
}

// Exec runs a statement and records the stack of the caller on error.
func (d *StackDriver) Exec(ctx context.Context, query string, args, v any) error {
	return errorsx.WithStack(d.Driver.Exec(ctx, query, args, v))
}

// Query runs a query and records the stack of the caller on error.
func (d *StackDriver) Query(ctx context.Context, query string, args, v any) error {
	return queryResult(d.Driver.Query(ctx, query, args, v), v)
}

// Tx starts a transaction whose errors record the stack of the caller.
func (d *StackDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	return d.BeginTx(ctx, nil)
}

// BeginTx starts a transaction with options. The generated ent client calls it
// through an interface check, so the wrapper must keep it.
func (d *StackDriver) BeginTx(ctx context.Context, opts *entsql.TxOptions) (dialect.Tx, error) {
	tx, err := d.Driver.BeginTx(ctx, opts)
	if err != nil {
		return nil, errorsx.WithStack(err)
	}

	return &stackTx{Tx: tx}, nil
}

// stackTx is a transaction that records the stack of the caller on each error.
type stackTx struct {
	// Tx is the ent transaction that runs the queries.
	dialect.Tx
}

// Exec runs a statement and records the stack of the caller on error.
func (t *stackTx) Exec(ctx context.Context, query string, args, v any) error {
	return errorsx.WithStack(t.Tx.Exec(ctx, query, args, v))
}

// Query runs a query and records the stack of the caller on error.
func (t *stackTx) Query(ctx context.Context, query string, args, v any) error {
	return queryResult(t.Tx.Query(ctx, query, args, v), v)
}

// Commit commits the transaction and records the stack of the caller on error.
func (t *stackTx) Commit() error {
	return errorsx.WithStack(t.Tx.Commit())
}

// Rollback aborts the transaction and records the stack of the caller on error.
func (t *stackTx) Rollback() error {
	return errorsx.WithStack(t.Tx.Rollback())
}

// queryResult records the stack of the caller on a query error. On success it
// also wraps the result rows: Query returns once Postgres accepts the query, and
// ent then reads the rows through the rows handle, not through the driver. A
// failure during that read (a dropped connection, a timeout in a large result)
// would otherwise reach Sentry with no stack.
func queryResult(err error, v any) error {
	if err != nil {
		return errorsx.WithStack(err)
	}
	if rows, ok := v.(*entsql.Rows); ok {
		rows.ColumnScanner = stackRows{ColumnScanner: rows.ColumnScanner}
	}

	return nil
}

// stackRows records the stack of the caller on each error while it reads the result rows.
type stackRows struct {
	// ColumnScanner is the rows handle of the query.
	entsql.ColumnScanner
}

// Scan copies the current row and records the stack of the caller on error.
func (r stackRows) Scan(dest ...any) error {
	return errorsx.WithStack(r.ColumnScanner.Scan(dest...))
}

// Err returns the error of the row iteration with the stack of the caller.
func (r stackRows) Err() error {
	return errorsx.WithStack(r.ColumnScanner.Err())
}

// Close closes the rows and records the stack of the caller on error.
func (r stackRows) Close() error {
	return errorsx.WithStack(r.ColumnScanner.Close())
}
