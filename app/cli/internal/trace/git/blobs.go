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

package git

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// BlobReader reads files at commits through one long-running
// "git cat-file --batch" process, so that many reads cost one process, not
// several processes each. It is not safe for concurrent use.
type BlobReader struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
	// err is the first failure to talk to git. The output may then be out of
	// step with the requests, so every later read fails too.
	err error
}

// NewBlobReader starts a BlobReader for the repository at repoRoot. Close it
// when done.
func NewBlobReader(repoRoot string) (*BlobReader, error) {
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = repoRoot
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file stdin: %w", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start git cat-file: %w", err)
	}

	return &BlobReader{cmd: cmd, in: in, out: bufio.NewReader(out)}, nil
}

// FileAt returns the content of path, repo-relative with forward slashes, at
// rev. It returns nil when rev does not resolve or does not hold path as a
// file.
func (r *BlobReader) FileAt(rev, path string) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if strings.ContainsAny(rev+path, "\n") {
		return nil, fmt.Errorf("cannot read %q at %q: name holds a newline", path, rev)
	}

	content, err := r.read(rev + ":" + path)
	if err != nil {
		r.err = err
	}

	return content, err
}

func (r *BlobReader) read(name string) ([]byte, error) {
	if _, err := fmt.Fprintf(r.in, "%s\n", name); err != nil {
		return nil, fmt.Errorf("write git cat-file request: %w", err)
	}

	header, err := r.out.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read git cat-file header: %w", err)
	}
	header = strings.TrimSuffix(header, "\n")
	// "<name> missing" or "<name> ambiguous": the name can hold spaces.
	if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
		return nil, nil
	}

	// "<oid> <type> <size>", then the content and a newline.
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return nil, fmt.Errorf("unexpected git cat-file header %q", header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("unexpected git cat-file header %q: %w", header, err)
	}
	content := make([]byte, size+1)
	if _, err := io.ReadFull(r.out, content); err != nil {
		return nil, fmt.Errorf("read git cat-file content: %w", err)
	}
	if fields[1] != "blob" {
		return nil, nil
	}

	return content[:size], nil
}

// Close stops the git process.
func (r *BlobReader) Close() error {
	_ = r.in.Close()

	return r.cmd.Wait()
}
