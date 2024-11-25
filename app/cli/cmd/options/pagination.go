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

package options

import "github.com/spf13/cobra"

type PaginationOpts struct {
	// Number of items to show
	Limit                  int
	DefaultLimit           int
	NextCursor, PrevCursor string
}

var _ Interface = (*PaginationOpts)(nil)

func (o *PaginationOpts) AddFlags(cmd *cobra.Command) {
	defaultLimit := 10
	if o.DefaultLimit != 0 {
		defaultLimit = o.DefaultLimit
	}

	cmd.PersistentFlags().IntVar(&o.Limit, "limit", defaultLimit, "number of items to show")
	cmd.PersistentFlags().StringVar(&o.NextCursor, "next", "", "cursor to load the next page")
}

type OffsetPaginationOpts struct {
	DefaultLimit int
	DefaultPage  int
	Limit        int
	Page         int
}

var _ Interface = (*OffsetPaginationOpts)(nil)

func (o *OffsetPaginationOpts) AddFlags(cmd *cobra.Command) {
	defaultLimit := 50
	if o.DefaultLimit != 0 {
		defaultLimit = o.DefaultLimit
	}

	defaultPage := 1
	if o.DefaultPage != 0 {
		defaultPage = o.DefaultPage
	}

	cmd.PersistentFlags().IntVar(&o.Limit, "limit", defaultLimit, "number of items to show")
	cmd.PersistentFlags().IntVar(&o.Page, "page", defaultPage, "page number")
}
