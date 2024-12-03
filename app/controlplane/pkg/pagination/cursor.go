//
// Copyright 2024 The Chainloop Authors.
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

package pagination

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultCursorLimit is the default number of items per page for cursor-based pagination
const DefaultCursorLimit = 10

// Cursor is a struct that holds the timestamp and identifier of a record
type Cursor struct {
	Timestamp *time.Time
	ID        *uuid.UUID
}

// CursorOptions is a struct that holds the cursor and limit for pagination
type CursorOptions struct {
	Cursor *Cursor
	Limit  int
}

// CursorPaginationError is the error type for cursor-based pagination
type CursorPaginationError struct {
	err error
}

// NewCursorPaginationError creates a new CursorPaginationError with the provided error
func NewCursorPaginationError(err error) CursorPaginationError {
	return CursorPaginationError{err}
}

// Error returns the error message
func (e CursorPaginationError) Error() string {
	return e.err.Error()
}

// IsCursorPaginationError checks if the error is an CursorPaginationError
func IsCursorPaginationError(err error) bool {
	return errors.As(err, &CursorPaginationError{})
}

func NewCursor(c string, l int) (*CursorOptions, error) {
	var cursor *Cursor
	var limit = DefaultCursorLimit

	if c != "" {
		timestamp, id, err := decodeCursor(c)
		if err != nil {
			return nil, NewCursorPaginationError(fmt.Errorf("decoding cursor: %w", err))
		}
		cursor = &Cursor{Timestamp: timestamp, ID: id}
	}

	if l > 0 {
		limit = l
	}

	return &CursorOptions{
		Cursor: cursor,
		Limit:  limit,
	}, nil
}

// decodeCursor decodes a base64 encoded cursor into a timestamp and identifier
func decodeCursor(encodedCursor string) (*time.Time, *uuid.UUID, error) {
	byt, err := base64.StdEncoding.DecodeString(encodedCursor)
	if err != nil {
		return nil, nil, err
	}

	arrStr := strings.Split(string(byt), ",")
	if len(arrStr) != 2 {
		return nil, nil, errors.New("cursor is invalid")
	}

	timestamp, err := time.Parse(time.RFC3339Nano, arrStr[0])
	if err != nil {
		return nil, nil, err
	}

	rawID := arrStr[1]
	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, nil, err
	}

	return &timestamp, &id, nil
}

// EncodeCursor generates a base64 encoded representation of a timestamp + identifier
func EncodeCursor(t time.Time, uuid uuid.UUID) string {
	key := fmt.Sprintf("%s,%s", t.Format(time.RFC3339Nano), uuid)
	return base64.StdEncoding.EncodeToString([]byte(key))
}
