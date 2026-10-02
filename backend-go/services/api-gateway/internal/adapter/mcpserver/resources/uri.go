package resources

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
)

// Kind identifies a resource family (and the "resource:<kind>" policy name).
type Kind string

const (
	KindProjects       Kind = "projects"
	KindProject        Kind = "project"
	KindTask           Kind = "task"
	KindWorktreeStatus Kind = "worktree_status"
	KindWorktreeDiff   Kind = "worktree_diff"
	KindWorktreeFile   Kind = "worktree_file"
	KindReview         Kind = "review"
)

// errBadURI is the only parse error: malformed and unknown URIs are
// indistinguishable from "not found" to the caller.
var errBadURI = errors.New("resources: bad uri")

const (
	scheme      = "orca://"
	maxURILen   = 2048
	maxPathSize = 1024
)

// Ref is a parsed, validated resource URI.
type Ref struct {
	Kind Kind
	// ID is the project / task / worktree id (lower-cased UUID).
	ID string
	// Provider/Repo/Number are set for KindReview (Repo is percent-decoded).
	Provider, Repo, Number string
	// Path is the decoded worktree-relative path (KindWorktreeFile) or diff path.
	Path string
	// Query options.
	Base, Staged       string
	Offset, Length     int64
	hasOffset, hasLen  bool
	RawURI, CanonicalK string
}

var (
	uuidRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numberRe   = regexp.MustCompile(`^[0-9]{1,9}$`)
	repoPartRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,100}$`)
	refNameRe  = regexp.MustCompile(`^[A-Za-z0-9_./\-]{1,200}$`)
)

// Parse validates uri strictly. Raw URIs must be printable ASCII (anything
// else has to arrive percent-encoded); every component is percent-decoded
// exactly once and then validated, never "cleaned".
func Parse(uri string) (Ref, error) {
	if len(uri) == 0 || len(uri) > maxURILen || !strings.HasPrefix(uri, scheme) {
		return Ref{}, errBadURI
	}
	for i := 0; i < len(uri); i++ {
		if c := uri[i]; c <= 0x20 || c >= 0x7f {
			return Ref{}, errBadURI
		}
	}
	rest := uri[len(scheme):]
	if strings.ContainsAny(rest, "#@") {
		return Ref{}, errBadURI
	}
	rest, rawQuery, hasQuery := strings.Cut(rest, "?")
	if hasQuery && rawQuery == "" {
		return Ref{}, errBadURI
	}
	segs := strings.Split(rest, "/")
	ref := Ref{RawURI: uri}
	var err error
	switch {
	case len(segs) == 1 && segs[0] == "projects":
		ref.Kind = KindProjects
	case len(segs) == 2 && segs[0] == "project":
		ref.Kind = KindProject
		ref.ID, err = parseID(segs[1])
	case len(segs) == 2 && segs[0] == "task":
		ref.Kind = KindTask
		ref.ID, err = parseID(segs[1])
	case len(segs) == 3 && segs[0] == "worktree" && segs[2] == "status":
		ref.Kind = KindWorktreeStatus
		ref.ID, err = parseID(segs[1])
	case len(segs) == 3 && segs[0] == "worktree" && segs[2] == "diff":
		ref.Kind = KindWorktreeDiff
		ref.ID, err = parseID(segs[1])
	case len(segs) >= 4 && segs[0] == "worktree" && segs[2] == "file":
		ref.Kind = KindWorktreeFile
		ref.ID, err = parseID(segs[1])
		if err == nil {
			ref.Path, err = decodeOnce(strings.Join(segs[3:], "/"))
		}
	case len(segs) == 4 && segs[0] == "review":
		ref.Kind = KindReview
		err = parseReview(&ref, segs[1], segs[2], segs[3])
	default:
		err = errBadURI
	}
	if err != nil {
		return Ref{}, errBadURI
	}
	if err := parseQuery(&ref, rawQuery); err != nil {
		return Ref{}, errBadURI
	}
	switch ref.Kind {
	case KindWorktreeFile:
		if clean, err := tools.CleanWorktreePath(ref.Path); err != nil || len(clean) > maxPathSize {
			return Ref{}, errBadURI
		}
	case KindWorktreeDiff:
		// The diff path comes from the query and is required: git.diff is per-file.
		if _, err := tools.CleanWorktreePath(ref.Path); err != nil {
			return Ref{}, errBadURI
		}
	}
	ref.CanonicalK = canonicalKey(ref)
	return ref, nil
}

func canonicalKey(r Ref) string {
	switch r.Kind {
	case KindProjects:
		return "projects"
	case KindReview:
		return "review/" + r.Provider + "/" + r.Repo + "/" + r.Number
	case KindWorktreeFile:
		return "worktree/" + r.ID + "/file/" + r.Path
	default:
		return string(r.Kind) + "/" + r.ID + "/" + r.Path + "/" + r.Base + "/" + r.Staged
	}
}

func parseID(seg string) (string, error) {
	if !uuidRe.MatchString(seg) { // no '%' can match: ids are never decoded
		return "", errBadURI
	}
	return strings.ToLower(seg), nil
}

func decodeOnce(s string) (string, error) {
	d, err := url.PathUnescape(s)
	if err != nil {
		return "", errBadURI
	}
	return d, nil
}

func parseReview(ref *Ref, provider, repoSeg, number string) error {
	if provider != "github" && provider != "gitlab" {
		return errBadURI
	}
	if !numberRe.MatchString(number) {
		return errBadURI
	}
	repo, err := decodeOnce(repoSeg) // "org%2Fsub%2Frepo" -> one segment, decoded once
	if err != nil || repo == "" || len(repo) > 200 {
		return errBadURI
	}
	parts := strings.Split(repo, "/")
	if len(parts) > 10 || (provider == "github" && len(parts) != 2) || (provider == "gitlab" && len(parts) < 2) {
		return errBadURI
	}
	for _, p := range parts {
		if p == "." || p == ".." || !repoPartRe.MatchString(p) {
			return errBadURI
		}
	}
	ref.Provider, ref.Repo, ref.Number = provider, repo, number
	return nil
}

func parseQuery(ref *Ref, raw string) error {
	if raw == "" {
		if ref.Kind == KindWorktreeDiff {
			return errBadURI // path is required
		}
		return nil
	}
	allowed := map[Kind][]string{KindWorktreeDiff: {"path", "base", "staged"}, KindWorktreeFile: {"offset", "length"}}[ref.Kind]
	if len(allowed) == 0 {
		return errBadURI
	}
	seen := map[string]bool{}
	for _, pair := range strings.Split(raw, "&") {
		k, v, _ := strings.Cut(pair, "=")
		if seen[k] || !contains(allowed, k) {
			return errBadURI
		}
		seen[k] = true
		dv, err := url.QueryUnescape(v)
		if err != nil {
			return errBadURI
		}
		switch k {
		case "path":
			ref.Path = dv
		case "base":
			if !refNameRe.MatchString(dv) || strings.Contains(dv, "..") || strings.HasPrefix(dv, "-") {
				return errBadURI
			}
			ref.Base = dv
		case "staged":
			if dv != "true" && dv != "false" {
				return errBadURI
			}
			ref.Staged = dv
		case "offset", "length":
			n, err := strconv.ParseInt(dv, 10, 64)
			if err != nil || n < 0 || n > 1<<40 || (k == "length" && (n == 0 || n > 64<<10)) {
				return errBadURI
			}
			if k == "offset" {
				ref.Offset, ref.hasOffset = n, true
			} else {
				ref.Length, ref.hasLen = n, true
			}
		}
	}
	if ref.Kind == KindWorktreeDiff && ref.Path == "" {
		return errBadURI
	}
	if ref.Kind == KindWorktreeFile && ref.hasLen && !ref.hasOffset {
		ref.Offset, ref.hasOffset = 0, true
	}
	return nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
