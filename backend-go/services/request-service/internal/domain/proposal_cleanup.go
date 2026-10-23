package domain

import (
	"strconv"
	"strings"
	"time"
)

// DefaultProposalRetention is how long a worktree is kept when nothing else says to remove it: a failed run's, so a person
// can look at what the agent left, and a finished Request's, for the archive step.
const DefaultProposalRetention = 7 * 24 * time.Hour

// ProposalCleanup says what to do with the worktree of one OpenSpec change. The branch is never part of it: it stays so the
// proposal remains reviewable.
type ProposalCleanup struct {
	// Remove deletes the worktree. Abandon also marks the change abandoned (a cancelled Request will never use it).
	Remove, Abandon bool
}

// DecideProposalCleanup is the whole policy. runActive means a plan run of the Request is writing in the worktree right now,
// which is never touched.
func DecideProposalCleanup(change ChangeStatus, request RequestStatus, updatedAt, now time.Time, retention time.Duration, runActive bool) ProposalCleanup {
	if runActive {
		return ProposalCleanup{}
	}
	aged := now.Sub(updatedAt) >= retention
	switch {
	case change == ChangeStatusAbandoned, change == ChangeStatusArchived:
		return ProposalCleanup{Remove: true} // nothing will write there any more
	case request == RequestStatusCancelled:
		return ProposalCleanup{Remove: true, Abandon: true}
	case request == RequestStatusCompleted:
		return ProposalCleanup{Remove: aged}
	case change == ChangeStatusPreparing:
		return ProposalCleanup{Remove: aged} // a failed run's leftover; a retry creates the worktree again on the same branch
	}
	return ProposalCleanup{}
}

// ReleaseWorkspace records that the worktree is gone.
func (c *OpenSpecChange) ReleaseWorkspace() { c.WorkspaceID = "" }

// ProposalBranch is the local branch that holds the OpenSpec change of one Request. It is never pushed.
func ProposalBranch(number int64) string {
	return "request/" + strconv.FormatInt(number, 10) + "-proposal"
}

// ProposalBranchFor is the Request's own branch name, used when ProposalBranch is already taken by something no record of this
// Request knows: Request numbers start again at 1 after a database reset, and two projects can share one repository, so the
// number alone is not unique on disk. The first 8 hex digits of the Request id make it so.
func ProposalBranchFor(number int64, requestID string) string {
	return ProposalBranch(number) + "-" + RequestIDShort(requestID)
}

// IsProposalBranch says whether branch is one of the two names a Request's proposal can have.
func IsProposalBranch(branch string, number int64, requestID string) bool {
	return branch == ProposalBranch(number) || branch == ProposalBranchFor(number, requestID)
}

// RequestIDShort is the first 8 hex digits of a Request id (dashes dropped); the whole id when it is shorter.
func RequestIDShort(requestID string) string {
	hex := strings.ReplaceAll(requestID, "-", "")
	if len(hex) > 8 {
		return hex[:8]
	}
	return hex
}

// ProposalRetentionFromEnv reads REQUEST_PROPOSAL_RETENTION as a Go duration ("168h") or whole days ("7d"). A value that
// cannot be read, or is not positive, keeps the default and says so in the warning.
func ProposalRetentionFromEnv(getenv func(string) string) (time.Duration, string) {
	raw := strings.TrimSpace(getenv("REQUEST_PROPOSAL_RETENTION"))
	if raw == "" {
		return DefaultProposalRetention, ""
	}
	var d time.Duration
	if days, ok := strings.CutSuffix(raw, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return DefaultProposalRetention, "REQUEST_PROPOSAL_RETENTION " + strconv.Quote(raw) + " is not a duration; the default applies"
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(raw); err != nil {
			return DefaultProposalRetention, "REQUEST_PROPOSAL_RETENTION " + strconv.Quote(raw) + " is not a duration; the default applies"
		}
	}
	if d <= 0 {
		return DefaultProposalRetention, "REQUEST_PROPOSAL_RETENTION must be positive; the default applies"
	}
	return d, ""
}
