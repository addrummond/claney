package glob

import (
	"strings"
)

// The character which is treated like a glob
const GLOB = '*'

// Glob will test a string pattern, potentially containing globs, against a
// subject string. The result is a simple true/false, determining whether or
// not the glob pattern matched the subject text.
func Glob(pattern, subj string) bool {
	// Empty pattern can only match empty subject
	if pattern == "" {
		return subj == pattern
	}

	// If the pattern _is_ a glob, it matches everything
	if pattern == string(GLOB) {
		return true
	}

	parts := splitParts(pattern)

	leadingGlob := strings.HasPrefix(pattern, string(GLOB))
	trailingGlob := strings.HasSuffix(pattern, string(GLOB))
	end := len(parts) - 1

	// Go over the leading parts and ensure they match.
	for i := 0; i < end; i++ {
		idx := strings.Index(subj, parts[i])

		switch i {
		case 0:
			// Check the first section. Requires special handling.
			if !leadingGlob && idx != 0 {
				return false
			}
		default:
			// Check that the middle parts match.
			if idx < 0 {
				return false
			}
		}

		// Trim evaluated text from subj as we loop over the pattern.
		subj = subj[idx+len(parts[i]):]
	}

	// Reached the last section. Requires special handling.
	return trailingGlob || strings.HasSuffix(subj, parts[end])
}

// contains glob or escaped glob (so can't be matched just using string eq)
func IsNonLiteral(input string) bool {
	return strings.ContainsRune(input, GLOB)
}

// Could refactor this so that we don't alloc new strings and we use custom
// versions of strings.Index and strings.HasSuffix that can handle \* escapes.
// However, as globs are usually very small strings, it's probably not worth the
// bother.
func splitParts(input string) (parts []string) {
	var currentPart strings.Builder

	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+1 < len(input) && input[i+1] == GLOB {
			i++
			currentPart.WriteByte('*')
		} else if input[i] == GLOB {
			parts = append(parts, currentPart.String())
			currentPart.Reset()
			continue
		} else {
			currentPart.WriteByte(input[i])
		}
	}

	parts = append(parts, currentPart.String())

	return
}
