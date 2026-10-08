package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/apperrors"
)

const (
	ACStatusActive  = "active"
	ACStatusRetired = "retired"

	MaxAcceptanceCriteria = 50
	MaxACTextRunes        = 500
)

var acIDPattern = regexp.MustCompile(`^AC-([1-9][0-9]*)$`)

type AcceptanceCriterion struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Status     string `json:"status"`
	VerifyHint string `json:"verify_hint,omitempty"`
}

// AcceptanceCriteria numbers its items from Next, which only ever grows: a retired AC keeps its
// number so a Solution that cites REQ-142@r1 still reads the same criterion.
type AcceptanceCriteria struct {
	Items []AcceptanceCriterion
	Next  int
}

func ErrACInvalid(reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeArtifactSchemaInvalid, "acceptance criteria: "+reason, nil)
}

func ErrACLimitExceeded() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeArtifactLimitExceeded,
		fmt.Sprintf("a request holds at most %d acceptance criteria", MaxAcceptanceCriteria), nil)
}

func normalizeACText(text string) string { return strings.TrimSpace(NormalizeNFC(text)) }

func validVerifyHint(h string) bool {
	switch h {
	case "", "test", "metric", "manual", "review":
		return true
	}
	return false
}

// Add appends an active AC with the next number.
func (a *AcceptanceCriteria) Add(text, hint string) (AcceptanceCriterion, error) {
	text = normalizeACText(text)
	if n := utf8.RuneCountInString(text); n < 1 || n > MaxACTextRunes {
		return AcceptanceCriterion{}, ErrACInvalid(fmt.Sprintf("text must be 1..%d characters", MaxACTextRunes))
	}
	if !validVerifyHint(hint) {
		return AcceptanceCriterion{}, ErrACInvalid("verify_hint must be test|metric|manual|review")
	}
	if len(a.Items) >= MaxAcceptanceCriteria {
		return AcceptanceCriterion{}, ErrACLimitExceeded()
	}
	if a.Next < 1 {
		a.Next = a.maxNumber() + 1
	}
	ac := AcceptanceCriterion{ID: "AC-" + strconv.Itoa(a.Next), Text: text, Status: ACStatusActive, VerifyHint: hint}
	a.Next++
	a.Items = append(a.Items, ac)
	return ac, nil
}

// Retire marks an AC retired; it is never removed and Next is untouched.
func (a *AcceptanceCriteria) Retire(id string) error {
	for i := range a.Items {
		if a.Items[i].ID == id {
			a.Items[i].Status = ACStatusRetired
			return nil
		}
	}
	return ErrACInvalid("unknown id " + id)
}

func (a AcceptanceCriteria) Find(id string) (AcceptanceCriterion, bool) {
	for _, it := range a.Items {
		if it.ID == id {
			return it, true
		}
	}
	return AcceptanceCriterion{}, false
}

func (a AcceptanceCriteria) ActiveIDs() []string {
	var out []string
	for _, it := range a.Items {
		if it.Status == ACStatusActive {
			out = append(out, it.ID)
		}
	}
	return out
}

func (a AcceptanceCriteria) maxNumber() int {
	max := 0
	for _, it := range a.Items {
		if m := acIDPattern.FindStringSubmatch(it.ID); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > max {
				max = n
			}
		}
	}
	return max
}

// ACInput is one desired entry of an edited list: an empty ID means a new criterion.
type ACInput struct {
	ID         string `json:"id,omitempty"`
	Text       string `json:"text"`
	Status     string `json:"status,omitempty"`
	VerifyHint string `json:"verify_hint,omitempty"`
}

// Reconcile applies a full desired list on top of the current criteria: unknown ids are
// rejected, entries without an id are added, and current criteria missing from desired are retired
// (never deleted, so numbers are not reused).
func (a AcceptanceCriteria) Reconcile(desired []ACInput) (AcceptanceCriteria, error) {
	out := AcceptanceCriteria{Items: append([]AcceptanceCriterion(nil), a.Items...), Next: a.Next}
	if out.Next < 1 {
		out.Next = out.maxNumber() + 1
	}
	kept := map[string]bool{}
	for _, d := range desired {
		if d.ID == "" {
			if _, err := out.Add(d.Text, d.VerifyHint); err != nil {
				return AcceptanceCriteria{}, err
			}
			continue
		}
		idx := -1
		for i, it := range out.Items {
			if it.ID == d.ID {
				idx = i
			}
		}
		if idx < 0 {
			return AcceptanceCriteria{}, ErrACInvalid("unknown id " + d.ID)
		}
		if kept[d.ID] {
			return AcceptanceCriteria{}, ErrACInvalid("duplicate id " + d.ID)
		}
		kept[d.ID] = true
		cur := out.Items[idx]
		if cur.Status == ACStatusRetired {
			continue // a retired AC stays as it was; reviving it would rewrite history
		}
		text := normalizeACText(d.Text)
		if n := utf8.RuneCountInString(text); n < 1 || n > MaxACTextRunes {
			return AcceptanceCriteria{}, ErrACInvalid(fmt.Sprintf("text must be 1..%d characters", MaxACTextRunes))
		}
		if !validVerifyHint(d.VerifyHint) {
			return AcceptanceCriteria{}, ErrACInvalid("verify_hint must be test|metric|manual|review")
		}
		cur.Text, cur.VerifyHint = text, d.VerifyHint
		if d.Status == ACStatusRetired {
			cur.Status = ACStatusRetired
		}
		out.Items[idx] = cur
	}
	for i, it := range out.Items {
		if it.Status == ACStatusActive && !kept[it.ID] && containsID(a.Items, it.ID) {
			out.Items[i].Status = ACStatusRetired
		}
	}
	return out, nil
}

func containsID(items []AcceptanceCriterion, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}
