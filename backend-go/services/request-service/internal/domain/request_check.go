package domain

import (
	"encoding/json"
	"time"
)

type CheckKind string

const (
	CheckPerfBaseline    CheckKind = "perf_baseline"
	CheckPerfAfter       CheckKind = "perf_after"
	CheckTestsBefore     CheckKind = "tests_before"
	CheckTestsAfter      CheckKind = "tests_after"
	CheckSecurityRecheck CheckKind = "security_recheck"
	CheckOpsResult       CheckKind = "ops_result"
)

func AllCheckKinds() []CheckKind {
	return []CheckKind{CheckPerfBaseline, CheckPerfAfter, CheckTestsBefore, CheckTestsAfter, CheckSecurityRecheck, CheckOpsResult}
}

func (k CheckKind) Valid() bool {
	for _, v := range AllCheckKinds() {
		if v == k {
			return true
		}
	}
	return false
}

type CheckStatus string

const (
	CheckStatusPassed CheckStatus = "passed"
	CheckStatusFailed CheckStatus = "failed"
)

// CheckSource says who stands behind a measurement. orca_verified is reserved for results Orca measured
// itself (CR-REQ-029); no RPC can set it.
type CheckSource string

const (
	CheckSourceAgent        CheckSource = "agent"
	CheckSourceManual       CheckSource = "manual"
	CheckSourceOrcaVerified CheckSource = "orca_verified"
)

type RequestCheck struct {
	ID         string
	TenantID   string
	RequestID  string
	Kind       CheckKind
	Status     CheckStatus
	Metrics    json.RawMessage
	Summary    string
	Source     CheckSource
	TaskID     string
	RecordedBy string
	CreatedAt  time.Time
	// Effective marks the newest row of its kind; computed on read, never stored.
	Effective bool
}

// LatestCheck returns the effective row of kind. Ties on CreatedAt go to the later slice position,
// so callers pass rows in insertion order (repositories sort by created_at, id).
func LatestCheck(checks []RequestCheck, kind CheckKind) (RequestCheck, bool) {
	var best RequestCheck
	found := false
	for _, c := range checks {
		if c.Kind != kind {
			continue
		}
		if !found || !c.CreatedAt.Before(best.CreatedAt) {
			best, found = c, true
		}
	}
	return best, found
}

// MarkEffective sets Effective on the newest row of each kind and returns a copy.
func MarkEffective(checks []RequestCheck) []RequestCheck {
	out := make([]RequestCheck, len(checks))
	copy(out, checks)
	for _, kind := range AllCheckKinds() {
		latest, ok := LatestCheck(out, kind)
		if !ok {
			continue
		}
		for i := range out {
			if out[i].Kind == kind {
				out[i].Effective = out[i].ID == latest.ID
			}
		}
	}
	return out
}

// checkRule is the single source for where a kind may be recorded.
type checkRule struct {
	reqType  RequestType
	statuses []RequestStatus
}

var checkRules = map[CheckKind]checkRule{
	CheckPerfBaseline:    {RequestTypePerformance, []RequestStatus{RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval, RequestStatusPlanning}},
	CheckPerfAfter:       {RequestTypePerformance, []RequestStatus{RequestStatusExecuting}},
	CheckTestsBefore:     {RequestTypeRefactor, []RequestStatus{RequestStatusExecuting}},
	CheckTestsAfter:      {RequestTypeRefactor, []RequestStatus{RequestStatusExecuting}},
	CheckSecurityRecheck: {RequestTypeSecurity, []RequestStatus{RequestStatusExecuting}},
	CheckOpsResult:       {RequestTypeOpsRequest, []RequestStatus{RequestStatusExecuting}},
}

// CheckAllowedNow reports whether a check of kind may be recorded for a Request of this type and status.
func CheckAllowedNow(kind CheckKind, reqType RequestType, status RequestStatus) bool {
	rule, ok := checkRules[kind]
	if !ok || rule.reqType != reqType {
		return false
	}
	for _, s := range rule.statuses {
		if s == status {
			return true
		}
	}
	return false
}
