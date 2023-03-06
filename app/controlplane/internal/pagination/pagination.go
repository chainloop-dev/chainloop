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

package pagination

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Options struct {
	Cursor *Cursor
	Limit  int
}

type Cursor struct {
	Timestamp *time.Time
	ID        *uuid.UUID
}

const defaultLimit = 10

func New(c string, l int) (*Options, error) {
	var cursor *Cursor
	var limit = defaultLimit

	if c != "" {
		time, uuid, err := decodeCursor(c)
		if err != nil {
			return nil, fmt.Errorf("decoding cursor: %w", err)
		}
		cursor = &Cursor{Timestamp: time, ID: uuid}
	}

	if l > 0 {
		limit = l
	}

	return &Options{
		Cursor: cursor,
		Limit:  limit,
	}, nil
}

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
	uuid, err := uuid.Parse(rawID)
	if err != nil {
		return nil, nil, err
	}

	return &timestamp, &uuid, nil
}

// EncodeCursor generates a base64 encoded representation of a timestamp + identifier
func EncodeCursor(t time.Time, uuid uuid.UUID) string {
	key := fmt.Sprintf("%s,%s", t.Format(time.RFC3339Nano), uuid)
	return base64.StdEncoding.EncodeToString([]byte(key))
}
