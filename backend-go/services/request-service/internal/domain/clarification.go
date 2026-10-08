package domain

import (
	"fmt"
	"time"
)

type ClarificationStatus string

const (
	ClarificationStatusOpen      ClarificationStatus = "open"
	ClarificationStatusAnswered  ClarificationStatus = "answered"
	ClarificationStatusExpired   ClarificationStatus = "expired"
	ClarificationStatusCancelled ClarificationStatus = "cancelled"
)

func AllClarificationStatuses() []ClarificationStatus {
	return []ClarificationStatus{ClarificationStatusOpen, ClarificationStatusAnswered, ClarificationStatusExpired, ClarificationStatusCancelled}
}

func ParseClarificationStatus(s string) (ClarificationStatus, error) {
	for _, c := range AllClarificationStatuses() {
		if string(c) == s {
			return c, nil
		}
	}
	return "", ErrClarificationStateNotAllowed("unknown clarification status " + s)
}

type ClarificationSource string

const (
	ClarificationSourceReadiness            ClarificationSource = "readiness"
	ClarificationSourceSolutionOpenQuestion ClarificationSource = "solution_open_question"
	ClarificationSourcePlanAssumption       ClarificationSource = "plan_assumption"
	ClarificationSourceTaskBlocked          ClarificationSource = "task_blocked"
	ClarificationSourceManual               ClarificationSource = "manual"
)

func AllClarificationSources() []ClarificationSource {
	return []ClarificationSource{ClarificationSourceReadiness, ClarificationSourceSolutionOpenQuestion,
		ClarificationSourcePlanAssumption, ClarificationSourceTaskBlocked, ClarificationSourceManual}
}

func ParseClarificationSource(s string) (ClarificationSource, error) {
	for _, c := range AllClarificationSources() {
		if string(c) == s {
			return c, nil
		}
	}
	return "", ErrClarificationStateNotAllowed("unknown clarification source " + s)
}

// SystemOnly sources are raised by the platform, never through the public RPC.
func (s ClarificationSource) SystemOnly() bool {
	return s == ClarificationSourceReadiness || s == ClarificationSourceTaskBlocked
}

type Clarification struct {
	ID                      string
	TenantID                string
	RequestID               string
	Seq                     int
	Source                  ClarificationSource
	SourceRef               string
	Status                  ClarificationStatus
	ResumeStatus            RequestStatus
	Round                   int
	AskedRequestRevision    int
	AnsweredRequestRevision *int
	DueAt                   time.Time
	RemindedAt              *time.Time
	CancelReason            string
	CreatedBy               string
	CreatedAt               time.Time
	AnsweredAt              *time.Time
	Version                 int64
	Questions               []ClarificationQuestion
	Assignees               []Principal
}

// DisplayID is CLR-<reqnum>.<seq>.
func (c Clarification) DisplayID(reqNum int64) string { return FormatClarificationID(reqNum, c.Seq) }

func (c Clarification) IsExpired(now time.Time) bool {
	return c.Status == ClarificationStatusOpen && !now.Before(c.DueAt)
}

func (c *Clarification) requireOpen() error {
	if c.Status != ClarificationStatusOpen {
		return ErrClarificationNotOpen(c.ID, c.Status)
	}
	return nil
}

func (c *Clarification) Answer(at time.Time, revision int) error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	c.Status, c.AnsweredAt, c.AnsweredRequestRevision = ClarificationStatusAnswered, &at, &revision
	return nil
}

func (c *Clarification) Expire(at time.Time) error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	c.Status = ClarificationStatusExpired
	return nil
}

func (c *Clarification) Cancel(reason string, at time.Time) error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	c.Status, c.CancelReason = ClarificationStatusCancelled, reason
	return nil
}

// ResumeStatusFor decides, from the flow and where the request stands, where it goes after the answer (CR-REQ-028 section 2.1).
func ResumeStatusFor(flow FlowDefinition, source ClarificationSource, from RequestStatus) (RequestStatus, error) {
	bad := func() (RequestStatus, error) {
		return "", ErrClarificationStateNotAllowed(fmt.Sprintf("a %s clarification cannot be raised from status %s", source, from))
	}
	switch source {
	case ClarificationSourceReadiness:
		if from != RequestStatusAwaitingTypeConfirmation {
			return bad()
		}
		if flow.AnalysisKind != AnalysisNone {
			return RequestStatusAnalyzing, nil
		}
		return RequestStatusPlanning, nil
	case ClarificationSourceSolutionOpenQuestion:
		if from == RequestStatusAwaitingAnalysisApproval || from == RequestStatusAnalyzing {
			return RequestStatusAnalyzing, nil
		}
	case ClarificationSourcePlanAssumption:
		if from == RequestStatusAwaitingPlanApproval || from == RequestStatusPlanning {
			return RequestStatusPlanning, nil
		}
	case ClarificationSourceTaskBlocked:
		if from == RequestStatusExecuting {
			return RequestStatusExecuting, nil
		}
	case ClarificationSourceManual:
		switch from {
		case RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval:
			return RequestStatusAnalyzing, nil
		case RequestStatusPlanning, RequestStatusAwaitingPlanApproval:
			return RequestStatusPlanning, nil
		case RequestStatusExecuting:
			return RequestStatusExecuting, nil
		}
	}
	return bad()
}

// ReturnStageForResume is the backlog stage used when a clarification times out or runs out of rounds.
func ReturnStageForResume(source ClarificationSource, resume RequestStatus) ReturnStage {
	if source == ClarificationSourceReadiness {
		return ReturnStageClassification
	}
	switch resume {
	case RequestStatusAnalyzing:
		return ReturnStageAnalysis
	case RequestStatusPlanning:
		return ReturnStagePlan
	}
	return ReturnStageTask
}

// DefaultDue holds proposed (unvalidated) deadlines; a policy table can replace this function later.
func DefaultDue(source ClarificationSource, urgency Urgency, now time.Time) time.Time {
	urgent := urgency == UrgencyUrgent
	switch source {
	case ClarificationSourceReadiness:
		if urgent {
			return now.Add(24 * time.Hour)
		}
		return now.Add(7 * 24 * time.Hour)
	case ClarificationSourceTaskBlocked:
		return now.Add(24 * time.Hour)
	}
	if urgent {
		return now.Add(8 * time.Hour)
	}
	return now.Add(72 * time.Hour)
}

const DefaultMaxClarificationRounds = 3
