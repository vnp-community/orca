package domain

import (
	"fmt"
	"regexp"
	"strings"
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
	Status    ChangeStatus
	SyncState SyncState
	Commit    string
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
