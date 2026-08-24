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

package accesschk

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// Projection is the JSON structure handed to the policy engine at evaluation
// time. It carries exactly the same information as a Report, but the security
// descriptors are de-duplicated: every distinct descriptor (the access-control
// portion of an object) is listed once in Descriptors, and each object references
// one by index. AccessChk evidence for a registry hive or a service database
// applies a handful of distinct descriptors to hundreds of thousands of objects
// through inheritance, so the flat form repeats the same DACL/ACE structures over
// and over. De-duplicating makes the projection — and the value the policy engine
// materialises from it — proportional to the number of DISTINCT descriptors
// rather than the number of objects, without dropping or altering any object,
// name, or ACE, so policy findings are unchanged.
//
// Policies read a descriptor via input.descriptors[obj.descriptor]. See the
// windows-*-strong-acls policies in the compliance-manifests repository.
type Projection struct {
	Tool        Tool              `json:"tool"`
	Descriptors []Descriptor      `json:"descriptors"`
	Objects     []ProjectedObject `json:"objects"`
	// Raw holds the full original text for inputs below RawRetentionLimit and is
	// empty otherwise, mirroring Report.Raw. It is a string-matching fallback and
	// is not read by current policies.
	Raw string `json:"raw"`
}

// Descriptor is the security-descriptor portion of an Object: the fields that are
// shared between objects with identical access control. The object name and the
// verbatim raw lines are intentionally excluded — they belong to the object, not
// the descriptor.
type Descriptor struct {
	DescriptorFlags []string      `json:"descriptor_flags,omitempty"`
	Owner           string        `json:"owner,omitempty"`
	DACL            []ACE         `json:"dacl,omitempty"`
	SACL            []ACE         `json:"sacl,omitempty"`
	AccessEntries   []AccessEntry `json:"access_entries"`
}

// ProjectedObject is a securable object in the de-duplicated projection: its name
// and an index into Projection.Descriptors, plus the verbatim RawLines fallback
// when retained (omitted for oversized inputs, matching Report).
type ProjectedObject struct {
	Name       string   `json:"name"`
	Descriptor int      `json:"descriptor"`
	RawLines   []string `json:"raw_lines,omitempty"`
}

// Project converts a parsed Report into its de-duplicated Projection. Two objects
// share a descriptor entry only when their descriptor fields are byte-for-byte
// identical, so no information is lost: every object keeps its own name and its
// exact descriptor, and the mapping is fully reconstructable.
func (r *Report) Project() (*Projection, error) {
	p := &Projection{
		Tool:        r.Tool,
		Descriptors: make([]Descriptor, 0),
		Objects:     make([]ProjectedObject, 0, len(r.Objects)),
		Raw:         r.Raw,
	}

	// index buckets descriptor positions by a cheap 64-bit fingerprint of their
	// canonical JSON. A fingerprint collision is resolved by a full structural
	// comparison, so distinct descriptors are never merged (no information lost).
	// The serialization reuses a single buffer instead of allocating a fresh key
	// per object, which for a hundreds-of-thousands-object material avoids the
	// same order of transient garbage as the whole flat projection.
	index := make(map[uint64][]int)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := range r.Objects {
		o := &r.Objects[i]
		d := Descriptor{
			DescriptorFlags: o.DescriptorFlags,
			Owner:           o.Owner,
			DACL:            o.DACL,
			SACL:            o.SACL,
			AccessEntries:   o.AccessEntries,
		}

		buf.Reset()
		if err := enc.Encode(&d); err != nil {
			return nil, err
		}
		fp := fnv64a(buf.Bytes())

		idx := -1
		for _, cand := range index[fp] {
			if reflect.DeepEqual(p.Descriptors[cand], d) {
				idx = cand
				break
			}
		}
		if idx < 0 {
			idx = len(p.Descriptors)
			p.Descriptors = append(p.Descriptors, d)
			index[fp] = append(index[fp], idx)
		}

		p.Objects = append(p.Objects, ProjectedObject{
			Name:       o.Name,
			Descriptor: idx,
			RawLines:   o.RawLines,
		})
	}

	return p, nil
}

// fnv64a is the 64-bit FNV-1a hash, inlined to fingerprint a byte slice without
// allocating a hash.Hash. It is used only to bucket candidate descriptors;
// equality is always confirmed structurally, so collisions are harmless.
func fnv64a(b []byte) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime
	}
	return h
}
