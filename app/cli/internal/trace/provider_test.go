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

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	tenOClock        = "2026-10-07T10:00:00Z"
	tenOClockPlusTwo = "2026-10-07T12:00:00+02:00"
)

func TestSkillUseRecordTime(t *testing.T) {
	cases := []struct {
		name string
		at   []string
		want string
	}{
		{name: "RFC3339 in UTC is kept", at: []string{tenOClock}, want: tenOClock},
		{name: "fractional seconds are dropped", at: []string{"2026-10-07T10:00:00.123Z"}, want: tenOClock},
		{name: "an offset is converted to UTC", at: []string{tenOClockPlusTwo}, want: tenOClock},
		{name: "an empty time is ignored", at: []string{""}, want: ""},
		{name: "an unreadable time is ignored", at: []string{"yesterday"}, want: ""},
		{name: "a readable time after an unreadable one is kept", at: []string{"yesterday", tenOClock}, want: tenOClock},
		{name: "an unreadable time does not replace a readable one", at: []string{tenOClock, "0"}, want: tenOClock},
		{
			// As raw strings, "2026-10-07T10:30:00Z" sorts before
			// tenOClockPlusTwo, which is 10:00 in UTC.
			name: "the earliest time wins across offsets",
			at:   []string{tenOClockPlusTwo, "2026-10-07T10:30:00Z"},
			want: tenOClock,
		},
		{name: "the earliest time wins in any order", at: []string{"2026-10-07T10:30:00.500Z", "2026-10-07T10:00:00.900Z"}, want: tenOClock},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var use SkillUse
			for _, at := range tc.at {
				use.Record(at, true, false)
			}

			assert.Equal(t, tc.want, use.FirstUsedAt)
			assert.Equal(t, len(tc.at), use.ByModel, "each use counts, also with no readable time")
		})
	}
}

func TestSkillUseMergeTime(t *testing.T) {
	cases := []struct {
		name  string
		first string
		other string
		want  string
	}{
		{name: "an earlier time of the other use wins", first: "2026-10-07T10:30:00Z", other: "2026-10-07T10:00:00.250Z", want: tenOClock},
		{name: "a later time of the other use is ignored", first: tenOClock, other: "2026-10-07T13:00:00+02:00", want: tenOClock},
		{name: "the other time fills an unknown time", first: "", other: tenOClockPlusTwo, want: tenOClock},
		{name: "an unreadable other time is ignored", first: tenOClock, other: "soon", want: tenOClock},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			use := SkillUse{Name: "review"}
			if tc.first != "" {
				use.Record(tc.first, true, false)
			}

			use.Merge(SkillUse{Name: "review", FirstUsedAt: tc.other, ByUser: 1})

			assert.Equal(t, tc.want, use.FirstUsedAt)
		})
	}
}

func TestSortedSkillUses(t *testing.T) {
	cases := []struct {
		name string
		uses []SkillUse
		want []string
	}{
		{
			name: "the order of first use",
			uses: []SkillUse{
				{Name: "b", FirstUsedAt: "2026-10-07T10:05:00Z"},
				{Name: "a", FirstUsedAt: "2026-10-07T10:10:00Z"},
			},
			want: []string{"b", "a"},
		},
		{
			name: "the name breaks ties",
			uses: []SkillUse{
				{Name: "b", FirstUsedAt: tenOClock},
				{Name: "a", FirstUsedAt: tenOClock},
			},
			want: []string{"a", "b"},
		},
		{
			name: "skills with no time come last, by name",
			uses: []SkillUse{
				{Name: "c"},
				{Name: "z", FirstUsedAt: "2026-10-07T10:10:00Z"},
				{Name: "a"},
				{Name: "y", FirstUsedAt: tenOClock},
			},
			want: []string{"y", "z", "a", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uses := make(map[string]*SkillUse, len(tc.uses))
			for i := range tc.uses {
				uses[tc.uses[i].Name] = &tc.uses[i]
			}

			var got []string
			for _, use := range SortedSkillUses(uses) {
				got = append(got, use.Name)
			}

			assert.Equal(t, tc.want, got)
		})
	}
}
