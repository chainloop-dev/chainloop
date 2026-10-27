//
// Copyright 2023-2026 The Chainloop Authors.
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
	"fmt"
)

func (c *Client) Ping() error {
	// The driver can be wrapped (for example to record error stacks), so ask
	// for the method instead of the concrete *sql.Driver type.
	d, ok := c.driver.(interface{ DB() *sql.DB })
	if !ok {
		return fmt.Errorf("ping: driver %T does not expose a database", c.driver)
	}

	return d.DB().Ping()
}
