package tools

import (
	"errors"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ErrUnsafePath is returned for every rejected worktree path. Callers must
// answer it like "not found" so the reason is never an oracle.
var ErrUnsafePath = errors.New("tools: unsafe worktree path")

// Any residual %XX after the single decode is a double-encoding attempt.
var doubleEncoded = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

// CleanWorktreePath validates a worktree-relative path that has ALREADY been
// percent-decoded exactly once. It does not "fix" anything: a path that needs
// cleaning is rejected, so what we authorize is byte-for-byte what git-gateway
// receives. Checks run on the raw value AND its NFKC form, which folds
// full-width and dot-leader look-alikes (U+FF0E, U+2024, U+FF0F...) into the
// ASCII characters they imitate.
func CleanWorktreePath(raw string) (string, error) {
	if raw == "" || len(raw) > 1024 || !utf8.ValidString(raw) { // overlong UTF-8 (%c0%ae) is invalid
		return "", ErrUnsafePath
	}
	for _, form := range []string{raw, norm.NFKC.String(raw)} {
		if err := checkPathForm(form); err != nil {
			return "", err
		}
	}
	return raw, nil
}

func checkPathForm(p string) error {
	if strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || doubleEncoded.MatchString(p) {
		return ErrUnsafePath
	}
	if len(p) >= 2 && p[1] == ':' && unicode.IsLetter(rune(p[0])) { // X:\ drive paths
		return ErrUnsafePath
	}
	for _, r := range p {
		// Controls (incl. NUL, C1), bidi overrides/isolates and invisible
		// format characters can disguise a name in logs and UIs.
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ErrUnsafePath
		}
	}
	for _, seg := range strings.Split(p, "/") {
		switch {
		case seg == "" || seg == "." || seg == "..":
			return ErrUnsafePath
		case strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " "): // Windows trims these
			return ErrUnsafePath
		}
	}
	return nil
}

// sensitiveDirs hold credentials: everything beneath is denied.
var sensitiveDirs = []string{".git", ".aws", ".ssh", ".gnupg", ".kube/config", ".docker/config.json"}

var sensitiveBase = []*regexp.Regexp{
	regexp.MustCompile(`^\.env$`),
	regexp.MustCompile(`^\.env\..+$`),
	regexp.MustCompile(`\.(pem|key|p12|pfx|keystore|jks)$`),
	regexp.MustCompile(`^id_(rsa|ed25519|ecdsa|dsa)`),
	regexp.MustCompile(`^(\.npmrc|\.pypirc|\.netrc|\.git-credentials|\.htpasswd|credentials|credentials\.json)$`),
	regexp.MustCompile(`\.tfstate`),
	regexp.MustCompile(`^secrets?\.`),
	regexp.MustCompile(`\.secrets?$`),
}

var envExampleSuffix = regexp.MustCompile(`\.(example|sample|template)$`)

// IsSensitivePath reports whether reading the file would expose credentials.
// rel must have passed CleanWorktreePath. Matching is case-insensitive and on
// the NFKC form (macOS/Windows file systems fold case). extra are tenant
// globs (path.Match) that can only widen the deny set.
func IsSensitivePath(rel string, extra []string) bool {
	p := strings.ToLower(norm.NFKC.String(rel))
	for _, d := range sensitiveDirs {
		if p == d || strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	segs := strings.Split(p, "/")
	for _, seg := range segs[:len(segs)-1] {
		for _, d := range []string{".git", ".aws", ".ssh", ".gnupg"} { // nested repos/submodules
			if seg == d {
				return true
			}
		}
	}
	base := segs[len(segs)-1]
	if strings.HasPrefix(base, ".env") && envExampleSuffix.MatchString(base) {
		return false
	}
	for _, re := range sensitiveBase {
		if re.MatchString(base) {
			return true
		}
	}
	for _, g := range extra {
		g = strings.ToLower(strings.TrimSpace(g))
		if g == "" {
			continue
		}
		if ok, _ := path.Match(g, p); ok {
			return true
		}
		if ok, _ := path.Match(g, base); ok {
			return true
		}
	}
	return false
}

var pemPrivateKey = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)

// ContainsPrivateKey is the after-read check for key material in a file whose
// name looked harmless.
func ContainsPrivateKey(s string) bool { return pemPrivateKey.MatchString(s) }
