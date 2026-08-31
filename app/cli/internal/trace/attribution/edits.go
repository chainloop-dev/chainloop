package attribution

import (
	"bytes"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

// ReverseApplyEdits reconstructs the "before" content of a file given its
// "after" content and the sequence of old_string → new_string replacements
// that were applied to produce it. Edits are reversed in reverse order.
//
// Best-effort and inherently ambiguous when new_string is not unique in
// the current content: bytes.Index always picks the first occurrence,
// which can be the wrong one if the same literal appears more than once.
// Cursor's afterFileEdit edits are typically context-rich enough that
// this rarely misfires in practice, and the worst-case outcome is a
// downstream line-range comparison reporting noisy ranges rather than a
// crash — but callers should treat the reconstructed bytes as a hint,
// not ground truth.
//
// If an edit's new_string cannot be located in the current content
// (because a later edit overlapped it, or the file was touched
// externally), that edit is skipped. When the resulting "before" equals
// "after" we return nil so callers can fall back to other reconstruction
// strategies.
func ReverseApplyEdits(after []byte, edits []trace.HookEdit) []byte {
	if len(edits) == 0 {
		return nil
	}

	result := make([]byte, len(after))
	copy(result, after)

	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		if e.NewString == "" {
			continue
		}

		newBytes := []byte(e.NewString)
		idx := bytes.Index(result, newBytes)
		if idx < 0 {
			continue
		}

		var b bytes.Buffer
		b.Grow(len(result) - len(newBytes) + len(e.OldString))
		b.Write(result[:idx])
		b.WriteString(e.OldString)
		b.Write(result[idx+len(newBytes):])
		result = b.Bytes()
	}

	if bytes.Equal(result, after) {
		return nil
	}

	return result
}
