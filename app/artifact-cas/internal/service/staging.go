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

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

// SweepStagingDir removes leftover upload staging files from a previous run
// (e.g. a crash mid-transfer that skipped the deferred cleanup). It only removes
// files carrying the staging prefix — anything else in the directory is left
// untouched — and is a no-op when the directory does not exist. It returns the
// number of files removed. Intended to run once at startup, before serving
// traffic, so the staging volume starts clean.
func SweepStagingDir(dir string, logger *log.Helper) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A not-yet-populated volume is fine; nothing to sweep.
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading staging dir %q: %w", dir, err)
	}

	var removed int
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), stagingFilePrefix) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.Remove(p); err != nil {
			// Best-effort: log and continue so one stuck file doesn't block boot.
			logger.Warnw("msg", "failed to remove leftover staging file", "path", p, "error", err.Error())
			continue
		}
		removed++
	}
	if removed > 0 {
		logger.Infow("msg", "swept leftover staging files", "dir", dir, "removed", removed)
	}
	return removed, nil
}
