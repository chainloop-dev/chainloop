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
	"errors"
)

const (
	// DefaultPage defines the default page number
	DefaultPage = 1
	// DefaultPageSize defines the default number of items per page
	DefaultPageSize = 15
)

// OffsetPaginationError is the error type for page-based pagination
type OffsetPaginationError struct {
	err error
}

// NewOffsetPaginationErrorStr creates a new OffsetPaginationError with the provided error message
func NewOffsetPaginationErrorStr(errMsg string) OffsetPaginationError {
	return OffsetPaginationError{errors.New(errMsg)}
}

// Error returns the error message
func (e OffsetPaginationError) Error() string {
	return e.err.Error()
}

// IsOffsetPaginationError checks if the error is an OffsetPaginationError
func IsOffsetPaginationError(err error) bool {
	return errors.As(err, &OffsetPaginationError{})
}

// Unwrap returns the wrapped error
func (e OffsetPaginationError) Unwrap() error {
	return e.err
}

// OffsetPaginationOpts is the options for page-based pagination
type OffsetPaginationOpts struct {
	// page is the page number
	page int
	// pageSize is the number of items per page
	pageSize int
}

// NewDefaultOffsetPaginationOpts creates a new OffsetPaginationOpts with default values
func NewDefaultOffsetPaginationOpts() *OffsetPaginationOpts {
	return &OffsetPaginationOpts{
		page:     DefaultPage,
		pageSize: DefaultPageSize,
	}
}

// Offset returns the page number
func (o *OffsetPaginationOpts) Offset() int {
	if o.page < 1 {
		o.page = 1
	}
	return (o.page - 1) * o.pageSize
}

// Limit returns the number of items per page
func (o *OffsetPaginationOpts) Limit() int {
	return o.pageSize
}

// NewOffsetPaginationOpts creates a new OffsetPaginationOpts with the provided page and pageSize
func NewOffsetPaginationOpts(offset, limit int) (*OffsetPaginationOpts, error) {
	if limit < 1 {
		return nil, NewOffsetPaginationErrorStr("pageSize must be greater than 0")
	}

	if offset < 0 {
		return nil, NewOffsetPaginationErrorStr("page must be greater than or equal to 0")
	}

	return &OffsetPaginationOpts{
		page:     offset,
		pageSize: limit,
	}, nil
}
