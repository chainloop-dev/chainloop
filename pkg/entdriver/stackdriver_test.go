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

package entdriver

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"github.com/chainloop-dev/chainloop/pkg/errorsx/errorsxtest"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/suite"
)

type StackDriverSuite struct {
	suite.Suite
	drv *StackDriver
}

func TestStackDriver(t *testing.T) {
	suite.Run(t, new(StackDriverSuite))
}

// errDBClosed is the message of the unexported database/sql error that a
// closed database returns, as when the backend shuts down.
const errDBClosed = "sql: database is closed"

// SetupTest opens a closed database, so that every query fails.
func (s *StackDriverSuite) SetupTest() {
	db, err := sql.Open("pgx", "postgres://localhost:1/unused")
	s.Require().NoError(err)
	s.Require().NoError(db.Close())

	s.drv = NewStackDriver(entsql.OpenDB(dialect.Postgres, db))
}

func queryAtOrigin(ctx context.Context, drv dialect.ExecQuerier) error {
	var rows entsql.Rows
	return drv.Query(ctx, "SELECT 1", []any{}, &rows)
}

func execAtOrigin(ctx context.Context, drv dialect.ExecQuerier) error {
	return drv.Exec(ctx, "SELECT 1", []any{}, nil)
}

// A database error records the stack of the query that failed, so that Sentry
// groups database failures by query instead of by the code that reports them.
// The message does not change, so logs and error checks do not change.
func (s *StackDriverSuite) TestDriverErrorsRecordCallerStack() {
	testCases := []struct {
		name        string
		originFunc  string
		runAtOrigin func(context.Context, dialect.ExecQuerier) error
	}{
		{name: "query", originFunc: "queryAtOrigin", runAtOrigin: queryAtOrigin},
		{name: "exec", originFunc: "execAtOrigin", runAtOrigin: execAtOrigin},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := tc.runAtOrigin(context.Background(), s.drv)
			s.Require().EqualError(err, errDBClosed)
			s.Contains(errorsxtest.OriginStack(errorsxtest.ErrorExceptions(err)), tc.originFunc)
		})
	}
}

// failingRows is a rows handle whose reads fail, as when the connection drops mid-result.
type failingRows struct {
	entsql.ColumnScanner
	err error
}

func (r failingRows) Scan(...any) error { return r.err }
func (r failingRows) Err() error        { return r.err }
func (r failingRows) Close() error      { return nil }

func scanAtOrigin(rows *entsql.Rows) error { return rows.Scan() }

func iterateAtOrigin(rows *entsql.Rows) error { return rows.Err() }

// A failure while ent reads the result rows from Postgres records the stack of
// the reader, like a failure of the query itself. The message does not change.
func (s *StackDriverSuite) TestRowReadErrorsRecordCallerStack() {
	readErr := errors.New("unexpected EOF")
	rows := &entsql.Rows{ColumnScanner: failingRows{err: readErr}}
	s.Require().NoError(queryResult(nil, rows))

	err := scanAtOrigin(rows)
	s.Require().ErrorIs(err, readErr)
	s.EqualError(err, readErr.Error())
	s.Contains(errorsxtest.OriginStack(errorsxtest.ErrorExceptions(err)), "scanAtOrigin")

	err = iterateAtOrigin(rows)
	s.Require().ErrorIs(err, readErr)
	s.Contains(errorsxtest.OriginStack(errorsxtest.ErrorExceptions(err)), "iterateAtOrigin")

	s.NoError(rows.Close(), "a successful Close stays nil")
}

// A transaction that cannot start records the stack of its caller too.
func (s *StackDriverSuite) TestTxErrorRecordsStack() {
	_, err := s.drv.Tx(context.Background())
	s.Require().EqualError(err, errDBClosed)
	s.True(errorsx.HasStack(err))
}
