package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrRepoAlreadyRegistered is what the repo adapters return (wrapped) when the
// unique registration_key index rejects a second registration of the same
// (tenant, dev server, path) — the race-proof backstop behind the usecase's
// own pre-check.
var ErrRepoAlreadyRegistered = errors.New("domain: repo location already registered")

// RepoConflict describes the repo that already occupies a location.
type RepoConflict struct {
	RepoID            string
	ProjectID         string
	ProjectName       string
	ProjectVisibility string
	URL               string
	DevServerID       string
}

// NormalizeRepoLocation reduces a repo url/path to the form used to decide
// "same repository on the same host". It only removes spelling noise (outer
// spaces, trailing and repeated separators); it never changes case or
// resolves "..", which would be wrong on case-sensitive hosts and across
// symlinks. The path style comes from the string itself, not from the OS
// project-service runs on, because the path lives on the dev server.
func NormalizeRepoLocation(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	switch {
	case strings.Contains(p, "://"):
		return strings.TrimRight(p, "/")
	case isWindowsPath(p):
		return normalizeWindowsPath(p)
	default:
		return normalizePosixPath(p)
	}
}

func isWindowsPath(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && isASCIILetter(p[0]) && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return len(p) == 2 && p[1] == ':' && isASCIILetter(p[0])
}

func isASCIILetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

func normalizePosixPath(p string) string {
	rooted := strings.HasPrefix(p, "/")
	parts := splitNonEmpty(p, "/")
	joined := strings.Join(parts, "/")
	if rooted {
		return "/" + joined
	}
	return joined
}

func normalizeWindowsPath(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	unc := strings.HasPrefix(p, `\\`)
	parts := splitNonEmpty(p, `\`)
	joined := strings.Join(parts, `\`)
	switch {
	case unc:
		return `\\` + joined
	case len(parts) == 1 && strings.HasSuffix(parts[0], ":"):
		return parts[0] + `\`
	default:
		return joined
	}
}

func splitNonEmpty(p, sep string) []string {
	var out []string
	for _, s := range strings.Split(p, sep) {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// RepoRegistrationKey is the value of repos.registration_key: a fixed-length
// digest so MySQL's index-key limit never matters. The tenant is hashed in so
// two tenants can never collide even if their dev server ids did. Returns ""
// (stored as NULL, never conflicting) when there is no path to key on.
func RepoRegistrationKey(tenantID, devServerID, url string) string {
	loc := NormalizeRepoLocation(url)
	if tenantID == "" || loc == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(tenantID + "\x00" + strings.ToLower(devServerID) + "\x00" + loc))
	return hex.EncodeToString(sum[:])
}
