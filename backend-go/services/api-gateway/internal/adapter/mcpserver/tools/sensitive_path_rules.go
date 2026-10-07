package tools

import (
	"path"
	"regexp"
	"strings"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/pathsafety"
	"golang.org/x/text/unicode/norm"
)

// ErrUnsafePath is returned for every rejected worktree path. Callers must
// answer it like "not found" so the reason is never an oracle.
var ErrUnsafePath = pathsafety.ErrUnsafePath

// CleanWorktreePath keeps the signature for resources/files guard.
func CleanWorktreePath(raw string) (string, error) {
	return pathsafety.CleanWorktreePath(raw)
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
