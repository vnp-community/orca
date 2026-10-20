package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/stablyai/orca-go/common/apperrors"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type ChangeStatus string

const (
	ChangeStatusPreparing ChangeStatus = "preparing"
	ChangeStatusReady     ChangeStatus = "ready"
	ChangeStatusArchived  ChangeStatus = "archived"
	ChangeStatusAbandoned ChangeStatus = "abandoned"
)

type SyncState string

const (
	SyncStateInSync  SyncState = "in_sync"
	SyncStatePending SyncState = "pending"
	SyncStateFailed  SyncState = "failed"
)

var ErrOpenSpecChangeStateInvalid = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_CHANGE_STATE_INVALID", "invalid change state transition", nil)

type OpenSpecChange struct {
	ID        string
	RequestID string
	// ChangeID is the directory name under openspec/changes/; it is fixed at the first generation of the Request.
	ChangeID  string
	Branch    string
	Status    ChangeStatus
	SyncState SyncState
	Commit    string
	// TasksMDDigest is PlanRegionDigest of the committed tasks.md.
	TasksMDDigest string
	// The knowledge fields are what the authoring agent reported it read; they are data for the reviewer, not facts Orca verified.
	KGAvailable bool
	KGSnapshot  string
	KGReadIDs   []string
	KGGaps      []string
	KGConflicts []string
	// SkillHash names the installed skill versions the agent worked under.
	SkillHash string
	// Bundle holds the committed files of the change by path relative to the change directory, so the proposal can be read
	// after its worktree is gone. Only text under the size caps is kept.
	Bundle map[string]string
	// RedactedSecrets counts the secrets masked in Bundle; the branch keeps the raw text. Rows written before CR-REQ-045 hold 0.
	RedactedSecrets int
	WorkspaceID     string
	// SyncDigest is the digest of the plan region Orca last wrote to tasks.md; a region that no longer matches was edited by hand.
	SyncDigest   string
	SyncError    string
	SyncAttempts int
	// SyncNextRetryAt is when a failed sync or archive is tried again; nil means as soon as the sweep comes by.
	SyncNextRetryAt *time.Time
	Version         int64
}

// SyncUpdate is the part of a change that the tasks.md sync and the archive step move, written at an expected version.
type SyncUpdate struct {
	State       SyncState
	Digest      string
	Error       string
	Attempts    int
	NextRetryAt *time.Time
}

// BundleDigest is a stable hash of the stored files: the plan gate folds it into its subject digest, so a change edited
// after the approver opened it no longer matches what was shown. It covers the masked copy; an approval opened before the
// masking existed hashed the raw text, and still matches because that row is never rewritten.
func (c OpenSpecChange) BundleDigest() string {
	paths := make([]string, 0, len(c.Bundle))
	for p := range c.Bundle {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%d:%s\n%d:%s\n", len(p), p, len(c.Bundle[p]), c.Bundle[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// omittedMarkerPattern is what the proposal run stores in place of a file that did not fit the bundle budget: the size and
// the sha256 of the raw file.
var omittedMarkerPattern = regexp.MustCompile(`^<omitted size=(\d+) sha256=([0-9a-f]{64})>$`)

// ParseOmittedMarker reads the size and hash a bundle file was replaced by; ok is false for any other text, or a size that
// does not fit an int64.
func ParseOmittedMarker(s string) (size int64, sha256 string, ok bool) {
	m := omittedMarkerPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, "", false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return n, m[2], true
}

func (c *OpenSpecChange) MarkReady() error {
	if c.Status != ChangeStatusPreparing {
		return ErrOpenSpecChangeStateInvalid
	}
	c.Status = ChangeStatusReady
	return nil
}

func (c *OpenSpecChange) MarkArchived(commit string) error {
	if c.Status != ChangeStatusReady {
		return ErrOpenSpecChangeStateInvalid
	}
	c.Status = ChangeStatusArchived
	c.Commit = commit
	return nil
}

func (c *OpenSpecChange) Abandon() error {
	if c.Status != ChangeStatusPreparing && c.Status != ChangeStatusReady {
		return ErrOpenSpecChangeStateInvalid
	}
	c.Status = ChangeStatusAbandoned
	return nil
}

// Reopen puts an abandoned change back to preparing when its Request needs a worktree again (a creation that failed released
// the marker, and the next run reserves it anew). A change in any other state is already in use.
func (c *OpenSpecChange) Reopen() error {
	if c.Status != ChangeStatusAbandoned {
		return ErrOpenSpecChangeStateInvalid
	}
	c.Status = ChangeStatusPreparing
	return nil
}

func removeMn() transform.Transformer {
	return runes.Remove(runes.In(unicode.Mn))
}

func NewChangeID(number int64, title string) string {
	if title == "" {
		return fmt.Sprintf("req-%d-request", number)
	}

	t := transform.Chain(norm.NFD, removeMn(), norm.NFC)
	s, _, _ := transform.String(t, title)

	s = strings.ReplaceAll(s, "đ", "d")
	s = strings.ReplaceAll(s, "Đ", "d")

	s = strings.ToLower(s)

	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	slug := b.String()
	re := regexp.MustCompile(`-+`)
	slug = re.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) == 0 {
		slug = "request"
	}
	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.TrimRight(slug, "-")
	}

	return fmt.Sprintf("req-%d-%s", number, slug)
}

var validChangeIDRegex = regexp.MustCompile(`^req-[0-9]+-[a-z0-9-]{1,40}$`)

func ValidChangeID(s string) bool {
	return validChangeIDRegex.MatchString(s)
}
