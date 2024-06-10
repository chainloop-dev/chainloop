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

package data

import "github.com/go-kratos/kratos/v2/log"

type DBStatus struct {
	data *Data
	log  *log.Helper
}

func NewDBStatus(data *Data, logger log.Logger) *DBStatus {
	return &DBStatus{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (s *DBStatus) Ping() error {
	return s.data.DB.Ping()
}
