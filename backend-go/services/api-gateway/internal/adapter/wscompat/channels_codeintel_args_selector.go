package wscompat

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/pathsafety"
)

type codeIntelSelector struct {
	ProjectID  string `json:"projectId"`
	WorktreeID string `json:"worktreeId"`
}

func (s codeIntelSelector) validate() error {
	if s.ProjectID == "" {
		return invalidParam("projectId", "required")
	}
	if len(s.ProjectID) > 64 {
		return invalidParam("projectId", "too_long")
	}
	if strings.IndexFunc(s.ProjectID, unicode.IsControl) >= 0 {
		return invalidParam("projectId", "control_characters_not_allowed")
	}

	if s.WorktreeID == "" {
		return invalidParam("worktreeId", "required")
	}
	if len(s.WorktreeID) > 512 {
		return invalidParam("worktreeId", "too_long")
	}
	if strings.IndexFunc(s.WorktreeID, unicode.IsControl) >= 0 {
		return invalidParam("worktreeId", "control_characters_not_allowed")
	}

	return nil
}

func toProtoSelector(s codeIntelSelector) *codeintelv1.WorktreeSelector {
	return &codeintelv1.WorktreeSelector{
		ProjectId:   s.ProjectID,
		WorktreeRef: s.WorktreeID,
	}
}

func checkBoundedInt(field string, v *int, lo, hi int) error {
	if v == nil {
		return nil
	}
	if *v < lo || *v > hi {
		return invalidParam(field, "out_of_range")
	}
	return nil
}

func checkEnum(field, v string, allowed ...string) error {
	if v == "" {
		return nil
	}
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return invalidParam(field, "invalid_enum")
}

func checkRelPath(field, v string) error {
	if v == "" {
		return nil
	}
	_, err := pathsafety.CleanWorktreePath(v)
	if err != nil {
		return pathNotAllowed(field)
	}
	return nil
}

var gitRefRegex = regexp.MustCompile(`^[\p{L}\p{N}_][\p{L}\p{N}._/@{}~^+-]*$`)

func checkGitRefLike(field, v string) error {
	if v == "" {
		return nil
	}
	if len(v) > 256 {
		return invalidParam(field, "too_long")
	}
	if strings.HasPrefix(v, "-") {
		return invalidParam(field, "invalid_git_ref")
	}
	if strings.Contains(v, "..") {
		return invalidParam(field, "invalid_git_ref")
	}
	if !gitRefRegex.MatchString(v) {
		return invalidParam(field, "invalid_git_ref")
	}
	return nil
}

func checkRFC3339(field, v string) error {
	if v == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, v); err != nil {
		return invalidParam(field, "invalid_rfc3339")
	}
	return nil
}

func checkStringMax(field, v string, n int) error {
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > n {
		return invalidParam(field, "too_long")
	}
	return nil
}

func checkPageToken(field, v string) error {
	if v == "" {
		return nil
	}
	if len(v) > 512 {
		return invalidParam(field, "too_long")
	}
	return nil
}

func checkOpaqueID(field, v string) error {
	if v == "" {
		return nil
	}
	if len(v) > 128 {
		return invalidParam(field, "too_long")
	}
	if strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return invalidParam(field, "control_characters_not_allowed")
	}
	return nil
}
