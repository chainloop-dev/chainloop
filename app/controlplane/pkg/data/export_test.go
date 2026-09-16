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

package data

// Internals exposed to the package's external tests. This file is only compiled during tests, so
// it does not widen the package's API. The visibility filters are reached from data_test because
// asserting on them against a real database needs the shared suite in biz/testhelpers, which
// imports this package and therefore cannot be imported from inside it.
var (
	ReferrerVisibleToOrgsForTest      = referrerVisibleToOrgs
	ProjectVisibilityPredicateForTest = projectVisibilityPredicate
)
