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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandCheckoutDirs(t *testing.T) {
	const cwd = "/work/home"
	t.Setenv("HOME", "/users/dev")

	cases := []struct {
		name string
		cmd  string
		want []string
	}{
		{name: "empty", cmd: "", want: nil},
		{name: "no directory", cmd: "make test", want: nil},
		{name: "cd absolute", cmd: "cd /tmp/repo-b && git commit -m x", want: []string{"/tmp/repo-b"}},
		{name: "cd relative to cwd", cmd: "cd sub && ls", want: []string{"/work/home/sub"}},
		{name: "cd relative to previous cd", cmd: "cd /a; cd b", want: []string{"/a", "/a/b"}},
		{name: "cd home", cmd: "cd ~/src/x && ls", want: []string{"/users/dev/src/x"}},
		{name: "cd on its own line", cmd: "cd /a\ngit commit", want: []string{"/a"}},
		{name: "cd in subshell", cmd: "(cd /a && make)", want: []string{"/a"}},
		{name: "pushd", cmd: "pushd /a >/dev/null && git status", want: []string{"/a"}},
		{name: "quoted path", cmd: `cd "/tmp/my repo" && git commit`, want: []string{"/tmp/my repo"}},
		{name: "single-quoted path", cmd: `cd '/tmp/my repo'`, want: []string{"/tmp/my repo"}},
		{name: "escaped space", cmd: `cd /tmp/my\ repo`, want: []string{"/tmp/my repo"}},
		{name: "cd text in an argument", cmd: `echo "cd /tmp/x"`, want: nil},
		{name: "comment", cmd: "ls # cd /tmp/x", want: nil},
		{name: "cd with variable", cmd: "cd $REPO && git commit", want: nil},
		{name: "relative cd after unknown cd", cmd: "cd $REPO && cd sub", want: nil},
		{name: "absolute cd after unknown cd", cmd: "cd $REPO && cd /a", want: []string{"/a"}},
		{name: "cd back", cmd: "cd - && ls", want: nil},
		{name: "duplicates", cmd: "cd /a && git -C /a commit", want: []string{"/a"}},
		{name: "git -C", cmd: `git -C /tmp/repo-b commit -m "x"`, want: []string{"/tmp/repo-b"}},
		{name: "git -C twice", cmd: "git -C /a -C b status", want: []string{"/a/b"}},
		{name: "git -c then -C", cmd: "git -c user.name=x -C /a commit", want: []string{"/a"}},
		{name: "env assignment", cmd: "GIT_TRACE=1 git -C /a status", want: []string{"/a"}},
		{name: "git without -C", cmd: "git commit -m x", want: nil},
		{name: "git clone with dir", cmd: "git clone git@github.com:o/repo-b.git /tmp/repo-b", want: []string{"/tmp/repo-b"}},
		{name: "git clone without dir", cmd: "git clone https://github.com/o/repo-b.git", want: []string{"/work/home/repo-b"}},
		{name: "git clone without dir, trailing slash", cmd: "git clone /srv/git/repo-b/", want: []string{"/work/home/repo-b"}},
		{name: "git clone after cd", cmd: "cd /tmp && git clone --depth 1 -b main https://github.com/o/repo-b", want: []string{"/tmp", "/tmp/repo-b"}},
		{name: "git clone option with equals", cmd: "git clone --branch=main https://github.com/o/repo-b dir", want: []string{"/work/home/dir"}},
		{name: "git clone with redirection", cmd: "git clone https://github.com/o/repo-b /tmp/b 2>&1 | tail -1", want: []string{"/tmp/b"}},
		{name: "git -C clone", cmd: "git -C /tmp clone https://github.com/o/repo-b", want: []string{"/tmp", "/tmp/repo-b"}},
		{name: "gh repo clone with dir", cmd: "gh repo clone o/repo-b /tmp/repo-b -- --depth 1", want: []string{"/tmp/repo-b"}},
		{name: "gh repo clone without dir", cmd: "gh repo clone o/repo-b", want: []string{"/work/home/repo-b"}},
		{name: "gh other command", cmd: "gh pr create --title x", want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, commandCheckoutDirs(tc.cmd, cwd))
		})
	}
}

func TestCommandCheckoutDirs_UnknownCwd(t *testing.T) {
	assert.Equal(t, []string{"/a"}, commandCheckoutDirs("cd sub && cd /a", ""))
}
