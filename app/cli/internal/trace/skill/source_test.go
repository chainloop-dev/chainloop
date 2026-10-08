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

package skill

import (
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

func TestSource(t *testing.T) {
	roots := Roots{
		Plugin:       []string{"/home/me/.claude/plugins"},
		Organization: []string{"/etc/claude-code/.claude/skills"},
		User:         []string{"/home/me/.claude/skills"},
		Project:      []string{"/home/me/src/repo"},
	}

	testCases := []struct {
		name      string
		skillName string
		dir       string
		want      string
	}{
		{name: "a plugin prefix", skillName: "superpowers:brainstorming", dir: "/somewhere/else", want: aicodingsession.SkillSourcePlugin},
		{name: "a plugin folder", skillName: "brainstorming", dir: "/home/me/.claude/plugins/cache/x/1.0.0/skills/brainstorming", want: aicodingsession.SkillSourcePlugin},
		{name: "a managed folder", skillName: "policy", dir: "/etc/claude-code/.claude/skills/policy", want: aicodingsession.SkillSourceOrganization},
		{name: "a user folder", skillName: "asd-ste100", dir: "/home/me/.claude/skills/asd-ste100", want: aicodingsession.SkillSourceUser},
		{name: "a repository folder", skillName: "release", dir: "/home/me/src/repo/.claude/skills/release", want: aicodingsession.SkillSourceProject},
		{name: "a folder next to the repository", skillName: "release", dir: "/home/me/src/repo-other/.claude/skills/release", want: aicodingsession.SkillSourceUnknown},
		{name: "a bundled skill", skillName: "claude-api", dir: "/tmp/bundled-skills/2.1.280/claude-api", want: aicodingsession.SkillSourceUnknown},
		{name: "no folder", skillName: "graphify", want: aicodingsession.SkillSourceUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Source(tc.skillName, tc.dir, roots))
		})
	}
}
