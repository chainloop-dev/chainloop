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

// Package pointer holds the parts of the pointers to spec sources that do not
// depend on the agent (spec issue-3556).
//
// A spec source is a file of the spec folder that the push stores as its own
// material. A session transcript can hold the same content again, for example
// in a full read of the file or in the write that created it. Each agent that
// can find these copies replaces each one, at its position, with a pointer
// that holds the digest of the material. Only exact copies are replaced: the
// digest of the content must be equal to the digest of a source.
package pointer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
)

// Marker is the type of a pointer, and the name of the field that holds a
// pointer in place of a text field. It tells any reader that Chainloop
// replaced the content.
const Marker = "chainloop.replaced"

// The finders, as the match report names them.
const (
	FinderFileRead  = "file_read"
	FinderFileWrite = "file_write"
)

// Reasons for a copy that stays inline, as the match report gives them.
const (
	ReasonNoSpecEntry = "no spec entry with this digest"
	ReasonFileGone    = "file gone"
	ReasonFileChanged = "file changed"
	ReasonPartialRead = "partial read"
	ReasonToolError   = "tool error"
	ReasonUnsupported = "unsupported result"
)

// Sources maps the digest of each spec source, as the file is on disk, to the
// digest of the material it is stored as. The two differ for a text file,
// whose material is its redacted copy.
type Sources map[string]string

// Digest returns the digest of content in the form that Sources uses.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return format(sum[:])
}

// DigestReader returns the digest of what r holds, in the form that Sources
// uses, with no copy of it in memory.
func DigestReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return format(h.Sum(nil)), nil
}

func format(sum []byte) string {
	return "sha256:" + hex.EncodeToString(sum)
}

// Report is the match report of one session, per finder.
type Report map[string]*Result

// Finder returns the result of a finder, which it adds when it is not there.
func (r Report) Finder(name string) *Result {
	res, ok := r[name]
	if !ok {
		res = &Result{}
		r[name] = res
	}

	return res
}

// Result counts the copies one finder looked at.
type Result struct {
	// Replaced is the number of copies replaced with a pointer.
	Replaced int
	// Skipped counts, per reason, the copies that stay inline.
	Skipped map[string]int
	// BytesRemoved is the size of the replaced content.
	BytesRemoved int
}

// Skip counts copies that stay inline for reason.
func (r *Result) Skip(reason string, copies int) {
	if copies == 0 {
		return
	}
	if r.Skipped == nil {
		r.Skipped = make(map[string]int)
	}
	r.Skipped[reason] += copies
}

// New returns the pointer to the material with the given digest.
func New(digest string) json.RawMessage {
	// A string always encodes.
	quoted, _ := json.Marshal(digest)
	return json.RawMessage(`{"type":"` + Marker + `","digest":` + string(quoted) + `}`)
}
