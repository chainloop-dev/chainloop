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
	const (
		cwd       = "/work/home"
		home      = "/users/dev"
		repoB     = "/tmp/repo-b"
		myRepo    = "/tmp/my repo"
		cwdRepoB  = "/work/home/repo-b"
		repoBURL  = "https://github.com/o/repo-b"
		otherRepo = "/a"
		otherSub  = "/a/b"
		tmp       = "/tmp"
	)
	t.Setenv("HOME", home)

	cases := []struct {
		name string
		cmd  string
		want []string
	}{
		{name: "empty", cmd: "", want: nil},
		{name: "no directory", cmd: "make test", want: nil},
		{name: "cd absolute", cmd: "cd " + repoB + " && git commit -m x", want: []string{repoB}},
		{name: "cd relative to cwd", cmd: "cd sub && ls", want: []string{"/work/home/sub"}},
		{name: "cd relative to previous cd", cmd: "cd /a; cd b", want: []string{otherRepo, otherSub}},
		{name: "cd home", cmd: "cd ~/src/x && ls", want: []string{"/users/dev/src/x"}},
		{name: "bare cd goes home", cmd: "cd && git clone " + repoBURL, want: []string{home, "/users/dev/repo-b"}},
		{name: "cd on its own line", cmd: "cd /a\ngit commit", want: []string{otherRepo}},
		{name: "cd in subshell", cmd: "(cd /a && make)", want: []string{otherRepo}},
		{name: "cd in subshell does not leak", cmd: "(cd /tmp) && git clone " + repoBURL, want: []string{tmp, cwdRepoB}},
		{name: "nested subshells", cmd: "(cd /a && (cd b) && git clone " + repoBURL + ")", want: []string{otherRepo, otherSub, "/a/repo-b"}},
		{name: "pushd", cmd: "pushd /a >/dev/null && git status", want: []string{otherRepo}},
		{name: "quoted path", cmd: `cd "` + myRepo + `" && git commit`, want: []string{myRepo}},
		{name: "single-quoted path", cmd: `cd '` + myRepo + `'`, want: []string{myRepo}},
		{name: "escaped space", cmd: `cd /tmp/my\ repo`, want: []string{myRepo}},
		{name: "cd text in an argument", cmd: `echo "cd /tmp/x"`, want: nil},
		{name: "comment", cmd: "ls # cd /tmp/x", want: nil},
		{name: "cd with variable", cmd: "cd $REPO && git commit", want: nil},
		{name: "relative cd after unknown cd", cmd: "cd $REPO && cd sub", want: nil},
		{name: "absolute cd after unknown cd", cmd: "cd $REPO && cd /a", want: []string{otherRepo}},
		{name: "cd back", cmd: "cd - && ls", want: nil},
		{name: "duplicates", cmd: "cd /a && git -C /a commit", want: []string{otherRepo}},
		{name: "git -C", cmd: `git -C ` + repoB + ` commit -m "x"`, want: []string{repoB}},
		{name: "git -C twice", cmd: "git -C /a -C b status", want: []string{otherSub}},
		{name: "git -c then -C", cmd: "git -c user.name=x -C /a commit", want: []string{otherRepo}},
		{name: "env assignment", cmd: "GIT_TRACE=1 git -C /a status", want: []string{otherRepo}},
		{name: "env assignment with expansion", cmd: `FOO="$HOME" git -C /a status`, want: []string{otherRepo}},
		{name: "git without -C", cmd: "git commit -m x", want: nil},
		{name: "git clone with dir", cmd: "git clone git@github.com:o/repo-b.git " + repoB, want: []string{repoB}},
		{name: "git clone without dir", cmd: "git clone " + repoBURL + ".git", want: []string{cwdRepoB}},
		{name: "git clone without dir, trailing slash", cmd: "git clone /srv/git/repo-b/", want: []string{cwdRepoB}},
		{name: "git clone after cd", cmd: "cd /tmp && git clone --depth 1 -b main " + repoBURL, want: []string{tmp, repoB}},
		{name: "git clone option with equals", cmd: "git clone --branch=main " + repoBURL + " dir", want: []string{"/work/home/dir"}},
		{name: "git clone with redirection", cmd: "git clone " + repoBURL + " /tmp/b 2>&1 | tail -1", want: []string{"/tmp/b"}},
		{name: "git -C clone", cmd: "git -C /tmp clone " + repoBURL, want: []string{tmp, repoB}},
		{name: "gh repo clone with dir", cmd: "gh repo clone o/repo-b " + repoB + " -- --depth 1", want: []string{repoB}},
		{name: "gh repo clone without dir", cmd: "gh repo clone o/repo-b", want: []string{cwdRepoB}},
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
