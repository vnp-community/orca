package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxWorktreeBaseRefBytes matches the base_ref column (VARCHAR(255) / CHECK char_length <= 255); counting bytes is the stricter bound.
const MaxWorktreeBaseRefBytes = 255

// WorktreeBaseRef is the branch or tag the worktrees of a Request start from. The empty value means "the repository default":
// it is resolved when a worktree is created, and no caller may replace it with the checked-out HEAD.
type WorktreeBaseRef string

// forbiddenRefRunes are the characters git check-ref-format refuses outright; each has its own rejection test.
const forbiddenRefRunes = `\~^:?*[`

// ParseWorktreeBaseRef validates a submitted ref. It is a safe subset of git check-ref-format, applied before anything reaches
// a git command line; git-gateway stays the final judge of whether the ref exists. "" is valid (use the default); a value that
// is only whitespace is not, because the caller typed something and it would silently turn into the default.
func ParseWorktreeBaseRef(raw string) (WorktreeBaseRef, error) {
	if raw == "" {
		return "", nil
	}
	ref := strings.TrimSpace(raw)
	if reason := baseRefViolation(ref); reason != "" {
		return "", ErrRequestBaseRefInvalid(reason)
	}
	return WorktreeBaseRef(ref), nil
}

// baseRefViolation returns the first broken rule, or "" when ref is acceptable.
func baseRefViolation(ref string) string {
	switch {
	case ref == "":
		return "must not be blank"
	case len(ref) > MaxWorktreeBaseRefBytes:
		return "must be at most 255 bytes"
	case !utf8.ValidString(ref):
		return "must be valid UTF-8"
	case strings.HasPrefix(ref, "-"):
		return "must not start with '-'"
	case ref == "@":
		return "must not be '@'"
	}
	for _, r := range ref {
		switch {
		case r == 0:
			return "must not contain a NUL byte"
		case unicode.IsControl(r):
			return "must not contain control characters"
		case unicode.IsSpace(r):
			return "must not contain whitespace"
		case strings.ContainsRune(forbiddenRefRunes, r):
			return "must not contain '" + string(r) + "'"
		}
	}
	return baseRefSequenceViolation(ref)
}

func baseRefSequenceViolation(ref string) string {
	switch {
	case strings.Contains(ref, ".."):
		return "must not contain '..'"
	case strings.Contains(ref, "@{"):
		return "must not contain '@{'"
	case strings.Contains(ref, "//"):
		return "must not contain '//'"
	case strings.HasPrefix(ref, "/"):
		return "must not start with '/'"
	case strings.HasSuffix(ref, "/"):
		return "must not end with '/'"
	case strings.HasSuffix(ref, "."):
		return "must not end with '.'"
	}
	for _, part := range strings.Split(ref, "/") {
		if strings.HasPrefix(part, ".") {
			return "a path component must not start with '.'"
		}
		if strings.HasSuffix(part, ".lock") {
			return "a path component must not end with '.lock'"
		}
	}
	return ""
}

// WorktreeBaseRefOrDefault is the single resolution rule for every worktree of the Request: the pinned ref, else the
// repository default the caller looked up. It never falls back to the HEAD of the main checkout.
func (r Request) WorktreeBaseRefOrDefault(defaultRef string) string {
	if r.BaseRef != "" {
		return r.BaseRef
	}
	return defaultRef
}
