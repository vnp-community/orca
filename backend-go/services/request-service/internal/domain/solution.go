package domain

import "time"

type SolutionKind string

const (
	SolutionKindSolution  SolutionKind = "solution"
	SolutionKindDiagnosis SolutionKind = "diagnosis"
	SolutionKindFindings  SolutionKind = "findings"
	SolutionKindAnswer    SolutionKind = "answer"
)

type SolutionStatus string

const (
	SolutionStatusDraft      SolutionStatus = "draft"
	SolutionStatusProposed   SolutionStatus = "proposed"
	SolutionStatusApproved   SolutionStatus = "approved"
	SolutionStatusRejected   SolutionStatus = "rejected"
	SolutionStatusSuperseded SolutionStatus = "superseded"
)

type Solution struct {
	ID              string
	TenantID        string
	RequestID       string
	Kind            SolutionKind
	Status          SolutionStatus
	OptionsJSON     []byte
	ChosenOption    *int
	ContentRef      string
	GenerationRunID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int64
	Options         []string
}
