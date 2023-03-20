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

package v1

import (
	"database/sql/driver"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Required to be stored in a byte[] database column with ent
// See https://entgo.io/docs/faq/#how-to-store-protobuf-objects-in-a-blob-column
// Implementation of scan interface
func (x *IntegrationConfig) Value() (driver.Value, error) {
	return proto.Marshal(x)
}

func (x *IntegrationConfig) Scan(src any) error {
	if src == nil {
		return nil
	}
	if b, ok := src.([]byte); ok {
		return proto.Unmarshal(b, x)
	}

	return fmt.Errorf("unexpected type %T", src)
}

// Implementation of scan interface
func (x *IntegrationAttachmentConfig) Value() (driver.Value, error) {
	return proto.Marshal(x)
}

func (x *IntegrationAttachmentConfig) Scan(src any) error {
	if src == nil {
		return nil
	}
	if b, ok := src.([]byte); ok {
		return proto.Unmarshal(b, x)
	}

	return fmt.Errorf("unexpected type %T", src)
}
