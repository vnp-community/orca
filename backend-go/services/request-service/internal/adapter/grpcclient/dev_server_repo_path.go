package grpcclient

import "strings"

// devServerRepoPath returns the repo URL when it is an absolute filesystem path on the dev server that runs the
// agent, and "" otherwise. project-service stores the folder path in Repo.URL for repos bound to a dev server;
// a remote URL (https://, git@host:) or a relative path names no folder to use as the agent's cwd, so the caller
// reports it instead of guessing one. The check is syntactic and OS-neutral because the dev server may be Windows
// or reached over SSH, so the backend host's own path rules do not apply.
func devServerRepoPath(url string) string {
	p := strings.TrimSpace(url)
	if p == "" || strings.ContainsAny(p, "\x00\r\n") || !isAbsoluteOnDevServer(p) {
		return ""
	}
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return ""
		}
	}
	return p
}

func isAbsoluteOnDevServer(p string) bool {
	switch {
	case p[0] == '/':
		return true
	case strings.HasPrefix(p, `\\`):
		return len(p) > 2 // UNC share
	case len(p) >= 3 && isASCIILetter(p[0]) && p[1] == ':' && (p[2] == '\\' || p[2] == '/'):
		return true
	}
	return false
}

func isASCIILetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
