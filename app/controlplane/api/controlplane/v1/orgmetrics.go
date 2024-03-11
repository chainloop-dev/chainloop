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

package v1

import "time"

func (tw MetricsTimeWindow) ToDuration() *time.Duration {
	var d time.Duration

	var (
		day     = 24 * time.Hour
		week    = 7 * day
		month   = 30 * day
		quarter = 3 * month
	)

	switch tw {
	case MetricsTimeWindow_METRICS_TIME_WINDOW_LAST_90_DAYS:
		d = quarter
	case MetricsTimeWindow_METRICS_TIME_WINDOW_LAST_30_DAYS:
		d = month
	case MetricsTimeWindow_METRICS_TIME_WINDOW_LAST_7_DAYS:
		d = week
	case MetricsTimeWindow_METRICS_TIME_WINDOW_LAST_DAY:
		d = day
	}

	return &d
}
