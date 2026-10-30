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
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const piSubagentCommit = "71023aec1546d6d677a65669c5665c0927ce0dd0"

// The parent writes and pushes after the child returns so the push snapshot
// contains the completed result, not an in-flight subagent invocation.
const piSubagentPrompt = `This is an automated test. Do exactly these steps in order:
1. Call the subagent tool with agent worker and agentScope user. Tell it to read README.md with its read tool and report the heading.
2. After the child returns, use your write tool to create ` + subagentFile + ` with the single line: from the subagent.
3. In a separate shell command run: git add ` + subagentFile + ` && git commit -m "add subagent file" && git push`

func (p *Pi) prepareSubagent(ctx context.Context, base string) (string, error) {
	version, err := p.Version(ctx)
	if err != nil {
		return "", err
	}
	if version != "1.3.0" {
		return "", fmt.Errorf("Pi subagent scenario requires 1.3.0, got %s", version)
	}
	bin, err := exec.LookPath(p.bin())
	if err != nil {
		return "", err
	}
	bin, err = filepath.EvalSymlinks(bin)
	if err != nil {
		return "", err
	}
	// npm ships these files beside dist/bundle/cli.js. Keep them there so
	// their imports resolve against the pinned package, not the workspace.
	dir := filepath.Join(filepath.Dir(bin), "..", "..", "examples", "extensions", "subagent")
	if err := verifyPiSubagentSource(dir); err != nil {
		return "", err
	}
	agentDir := filepath.Join(base, "pi-agent", "agents")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		return "", err
	}
	const worker = "---\nname: worker\ndescription: Read the requested file.\ntools: read\n---\nRead the requested file with the read tool, then report its heading. Do not modify files or start other agents.\n"
	if err := os.WriteFile(filepath.Join(agentDir, "worker.md"), []byte(worker), 0o600); err != nil {
		return "", err
	}
	return filepath.Join(dir, "index.ts"), nil
}

func verifyPiSubagentSource(dir string) error {
	for _, file := range []struct{ name, digest string }{
		{"index.ts", "1d11858d163e2a3a258af51c1923fc55ad825813acd6224462fd25ad61f5401b"},
		{"agents.ts", "cc85ecd33b89aa575bea3d2a9a9da7654a6305da4de02e5041aa5342a1de29b4"},
	} {
		data, err := os.ReadFile(filepath.Join(dir, file.name))
		if err != nil {
			return fmt.Errorf("read pinned Pi subagent source: %w", err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != file.digest {
			return fmt.Errorf("Pi subagent %s differs from pinned commit %s", file.name, piSubagentCommit)
		}
	}
	return nil
}
