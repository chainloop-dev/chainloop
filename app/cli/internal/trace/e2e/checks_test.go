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

package e2e

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
)

const (
	testAgentCommit = "aaaa"
	testHumanCommit = "bbbb"
)

// sessionWith returns a session whose stored evidence has the given commits
// and file changes. Its last commit is testHumanCommit. The evidence lists each
// commit with its subject, as the push writes it.
func sessionWith(commits []string, files ...aicodingsession.FileChange) *Session {
	s := &Session{Stored: &StoredSession{}, HeadSHA: testHumanCommit}
	d := &s.Stored.Evidence.Data
	d.GitContext = &aicodingsession.GitContext{}
	for _, c := range commits {
		d.GitContext.Commits = append(d.GitContext.Commits, c+" a subject")
	}
	d.CodeChanges = &aicodingsession.CodeChanges{Files: files}
	return s
}

// updatedGreeting is the file of the update scenario after the human commit
// replaced one of its three AI lines.
func updatedGreeting() aicodingsession.FileChange {
	return aicodingsession.FileChange{
		Path: greetingFile, Status: tracegit.StatusCreated, Attribution: trace.AttributionAI,
		LinesAdded: 3, AILinesAdded: 2, HumanLinesAdded: 1,
	}
}

func TestCheckNetContribution(t *testing.T) {
	cases := []struct {
		name    string
		file    aicodingsession.FileChange
		wantErr string
	}{
		{name: "replaced line no longer counts", file: updatedGreeting()},
		{
			name: "replaced line still counts as AI",
			file: func() aicodingsession.FileChange {
				f := updatedGreeting()
				f.AILinesAdded, f.HumanLinesAdded = 3, 0
				return f
			}(),
			wantErr: "3 AI lines",
		},
		{
			name: "line counted twice",
			file: func() aicodingsession.FileChange {
				f := updatedGreeting()
				f.LinesAdded = 4
				return f
			}(),
			wantErr: "4 lines added",
		},
		{
			name: "file no longer created",
			file: func() aicodingsession.FileChange {
				f := updatedGreeting()
				f.Status = "modified"
				return f
			}(),
			wantErr: "status",
		},
		{name: "file not listed", file: aicodingsession.FileChange{Path: "other.txt"}, wantErr: "do not list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkNetContribution(sessionWith([]string{testAgentCommit, testHumanCommit}, tc.file))
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCheckSessionUpdated(t *testing.T) {
	cases := []struct {
		name    string
		commits []string
		file    aicodingsession.FileChange
		wantErr string
	}{
		{name: "human commit joins the session", commits: []string{testAgentCommit, testHumanCommit}, file: updatedGreeting()},
		{name: "human commit not in the session", commits: []string{testAgentCommit}, file: updatedGreeting(), wantErr: "commit list"},
		{
			name:    "replacing line not human",
			commits: []string{testAgentCommit, testHumanCommit},
			file: func() aicodingsession.FileChange {
				f := updatedGreeting()
				f.HumanLinesAdded = 0
				return f
			}(),
			wantErr: "human lines",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSessionUpdated(sessionWith(tc.commits, tc.file))
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCheckAmended(t *testing.T) {
	amended := aicodingsession.FileChange{
		Path: greetingFile, Status: tracegit.StatusCreated, Attribution: trace.AttributionAI,
		LinesAdded: 4, AILinesAdded: 3, HumanLinesAdded: 1,
	}
	cases := []struct {
		name    string
		commits []string
		file    aicodingsession.FileChange
		wantErr string
	}{
		{name: "split follows the amend", commits: []string{testHumanCommit}, file: amended},
		{name: "amended commit not in the session", commits: []string{testAgentCommit}, file: amended, wantErr: "commit list"},
		{
			name:    "agent lines lost in the amend",
			commits: []string{testHumanCommit},
			file: func() aicodingsession.FileChange {
				f := amended
				f.AILinesAdded, f.HumanLinesAdded = 0, 4
				return f
			}(),
			wantErr: "0 AI lines",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAmended(sessionWith(tc.commits, tc.file))
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCheckRenamed(t *testing.T) {
	aiFile := func(path string) aicodingsession.FileChange {
		return aicodingsession.FileChange{
			Path: path, Status: tracegit.StatusCreated, Attribution: trace.AttributionAI, LinesAdded: 3, AILinesAdded: 3,
		}
	}
	cases := []struct {
		name    string
		file    aicodingsession.FileChange
		wantErr string
	}{
		{name: "listed under the path of the session commit", file: aiFile(greetingFile)},
		{name: "listed under the new path", file: aiFile(renamedFile)},
		{
			name: "lines lost in the rename",
			file: func() aicodingsession.FileChange {
				f := aiFile(renamedFile)
				f.AILinesAdded, f.HumanLinesAdded = 0, 3
				return f
			}(),
			wantErr: "0 AI lines",
		},
		{name: "file not listed", file: aiFile("other.txt"), wantErr: "do not list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRenamed(sessionWith(nil, tc.file))
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
