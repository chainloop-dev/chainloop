# Archive Detection Fix Report

**Date**: 2026-06-29  
**Branch**: zip-files  
**Commit**: (see below)

---

## Summary

Five fixes applied to the archive detection and extraction pipeline in Chainloop.

---

## Fix 1 (CRITICAL) — Non-file material values must not error in detection

### Problem
`detectByMagic` in `pkg/attestation/crafter/materials/archive.go` called `os.Open(path)` and returned a hard error when the file didn't exist. This broke material kinds like `STRING` and `CONTAINER_IMAGE` whose `--value` is not a file path.

### Before
```go
func detectByMagic(path string) (ArchiveFormat, error) {
    f, err := os.Open(path)
    if err != nil {
        return ArchiveNone, fmt.Errorf("opening %q: %w", path, err)
    }
    ...
}
```

### After
```go
func detectByMagic(path string) (ArchiveFormat, error) {
    f, err := os.Open(path)
    if err != nil {
        // If the file doesn't exist, the value is not a file path at all (e.g.
        // "hello world" for STRING or "registry/app:v1" for CONTAINER_IMAGE).
        // Treat it as a non-archive rather than propagating the error so callers
        // that pass non-file values are not surprised.
        return ArchiveNone, nil
    }
    ...
}
```

### TDD Evidence

**RED** (before fix):
```
=== RUN   TestShouldExplode/kind_STRING_non-file_value
    attestation_add_routing_test.go:73: 
        Error: Received unexpected error:
               opening "hello world": open hello world: no such file or directory
=== RUN   TestShouldExplode/kind_CONTAINER_IMAGE_non-file_value
    attestation_add_routing_test.go:73: 
        Error: Received unexpected error:
               opening "registry.example.com/app:v1": open registry.example.com/app:v1: no such file or directory
FAIL
```

**GREEN** (after fix):
```
=== RUN   TestShouldExplode/kind_STRING_non-file_value --- PASS
=== RUN   TestShouldExplode/kind_CONTAINER_IMAGE_non-file_value --- PASS
PASS
ok  github.com/chainloop-dev/chainloop/app/cli/pkg/action 0.026s
```

---

## Fix 2 (IMPORTANT) — safeArchivePath over-broad ".." rejection

### Problem
`safeArchivePath` used `strings.Contains(normalized, "..")` which rejected any path with `".."` as a substring — including legitimate filenames like `foo..bar.json`.

### Before
```go
func safeArchivePath(name string) bool {
    normalized := strings.ReplaceAll(name, "\\", "/")
    if strings.HasPrefix(normalized, "/") {
        return false
    }
    // Reject any path containing ".." which could escape the root
    if strings.Contains(normalized, "..") {
        return false
    }
    clean := path.Clean("/" + normalized)
    return !strings.Contains(clean, "/../") && clean != "/.."
}
```

### After
```go
func safeArchivePath(name string) bool {
    normalized := strings.ReplaceAll(name, "\\", "/")
    if strings.HasPrefix(normalized, "/") {
        return false
    }
    // Canonicalise against a virtual root and check that the result stays
    // within it. path.Clean will resolve ".." components so a path like
    // "a/../../etc/passwd" becomes "/etc/passwd" which does not start with
    // the virtual prefix "/root/"; a safe path like "a/b.txt" becomes
    // "/root/a/b.txt" which does.
    const root = "/root"
    clean := path.Clean(root + "/" + normalized)
    return strings.HasPrefix(clean, root+"/") || clean == root
}
```

### TDD Evidence

**RED** (before fix — `foo..bar.json` case):
```
=== RUN   TestSafeArchivePath/double_dot_in_filename_is_ok
    archive_test.go:189: 
        Error: Not equal: 
               expected: true
               actual  : false
FAIL
```

**GREEN** (after fix):
```
=== RUN   TestSafeArchivePath/double_dot_in_filename_is_ok --- PASS
=== RUN   TestSafeArchivePath/path_traversal --- PASS
=== RUN   TestSafeArchivePath/escape_via_nested_double_dot --- PASS
PASS
ok  github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials 0.012s
```

---

## Fix 3 (IMPORTANT) — Warn when --policy-input-from-file is used on the explode path

### Change
Added a warning log in `AttestationAdd.Run` (`app/cli/pkg/action/attestation_add.go`) inside the `if explode {` branch, before calling `AddMaterialsFromArchive`.

```go
if explode {
    if len(policyInputFiles) > 0 {
        action.Logger.Warn().Msg("--policy-input-from-file is ignored when expanding an archive; evidence cross-links are not recorded for exploded materials")
    }
    ...
}
```

---

## Fix 4 (MINOR) — Bound temp disk + avoid basename collision

### Problem
In `AddMaterialsFromArchive` (`pkg/attestation/crafter/crafter.go`), the temp file was named `filepath.Join(tmpDir, filepath.Base(name))`. Two archive entries with the same basename (e.g. `a/x.json` and `b/x.json`) would collide. Also, temp files were not removed until the deferred `os.RemoveAll(tmpDir)` at return.

### Before
```go
tmpPath := filepath.Join(tmpDir, filepath.Base(name))
// ... io.Copy ...
mt, err := c.stageMaterial(ctx, m, tmpPath, ...)
```

### After
```go
// Use the allocated unique material name for the temp file to avoid basename collisions.
tmpPath := filepath.Join(tmpDir, matName)
// ... io.Copy ...
mt, err := c.stageMaterial(ctx, m, tmpPath, ...)
// Remove the temp file immediately after staging to keep disk usage bounded.
os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
```

---

## Fix 5 (MINOR) — Add .tar / .tgz detection tests

Added `writeTar` helper (uncompressed tar, mirroring `writeTarGz` without the gzip layer) and two new test cases to `TestDetectArchive`:

- `{"tar by extension", tarPath, ArchiveTar}`
- `{"tgz by extension", tgzShortPath, ArchiveTarGz}`

---

## Test Results

### Focused tests
```
go test ./app/cli/pkg/action/ -run TestShouldExplode -v
PASS ok github.com/chainloop-dev/chainloop/app/cli/pkg/action 0.028s

go test ./pkg/attestation/crafter/materials/ -run 'TestDetectArchive|TestSafeArchivePath|TestWalkArchiveEntries' -v
PASS ok github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials 0.016s

SKIP_INTEGRATION=true go test ./pkg/attestation/crafter/ -run 'TestSuite/TestAddMaterialsFromArchive' -v
PASS ok github.com/chainloop-dev/chainloop/pkg/attestation/crafter 0.037s
```

### Full regression
```
SKIP_INTEGRATION=true go test ./pkg/attestation/crafter/... ./app/cli/...
ok  github.com/chainloop-dev/chainloop/pkg/attestation/crafter           1.529s
ok  github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials 2.643s
ok  github.com/chainloop-dev/chainloop/app/cli/pkg/action                0.048s
... (all packages pass)
```

### Build
```
go build ./app/cli/...  # exit 0 — no output
```

---

## Files Changed

1. `pkg/attestation/crafter/materials/archive.go` — Fix 1 (detectByMagic), Fix 2 (safeArchivePath)
2. `pkg/attestation/crafter/materials/archive_test.go` — Fix 2 new test cases, Fix 5 new detection tests
3. `app/cli/pkg/action/attestation_add.go` — Fix 3 (warning log)
4. `app/cli/pkg/action/attestation_add_routing_test.go` — Fix 1 new test cases
5. `pkg/attestation/crafter/crafter.go` — Fix 4 (temp file naming + cleanup)

---

## Self-Review

- Fix 1: The new `detectByMagic` behaviour is correct — if the value is not a file path, we silently return `ArchiveNone` which means `shouldExplode` returns `false, false, nil`. Non-archive kinds proceed through the normal add path.
- Fix 2: The virtual-root canonicalisation is the correct approach. `path.Clean("/root/" + name)` will resolve all `..` components against `/root`; if the result doesn't start with `/root/` the path escaped.
- Fix 3: Warning is placed before `AddMaterialsFromArchive` so it fires regardless of the archive content.
- Fix 4: `matName` is unique per entry by construction (NameAllocator), so no collision. `os.Remove` after staging keeps disk bounded; the `defer os.RemoveAll(tmpDir)` remains as a safety net.
- Fix 5: `writeTar` is a minimal helper that mirrors `writeTarGz` without the gzip wrapper.
