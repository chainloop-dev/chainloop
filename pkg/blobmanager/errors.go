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

package backend

import (
	"errors"
	"fmt"
)

type ErrNotFound struct {
	entity string
}

func NewErrNotFound(entity string) ErrNotFound {
	return ErrNotFound{entity}
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.entity)
}

func IsNotFound(err error) bool {
	return errors.As(err, &ErrNotFound{})
}

type ErrUploadSizeExceeded struct {
	want int64
	max  int64
}

func NewErrUploadSizeExceeded(want, maxSize int64) ErrUploadSizeExceeded {
	return ErrUploadSizeExceeded{want: want, max: maxSize}
}

func (e ErrUploadSizeExceeded) Error() string {
	return fmt.Sprintf("max size of upload exceeded: want=%d, max=%d", e.want, e.max)
}

func IsUploadSizeExceeded(err error) bool {
	return errors.As(err, &ErrUploadSizeExceeded{})
}
