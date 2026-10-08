package domain

import (
	"errors"
	"time"
)

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

var (
	ErrSolutionNotProposedState = errors.New("solution is not proposed")
	ErrSolutionNotDraft         = errors.New("solution is not a draft")
	ErrOptionIndexOutOfRange    = errors.New("option index out of range")
	ErrSolutionNotSupersedable  = errors.New("an approved solution cannot be superseded")
)

// KindForAnalysis maps the registry's analysis kind to the Solution kind it produces.
func KindForAnalysis(k AnalysisKind) (SolutionKind, bool) {
	switch k {
	case AnalysisSolution:
		return SolutionKindSolution, true
	case AnalysisDiagnosis:
		return SolutionKindDiagnosis, true
	case AnalysisFindings:
		return SolutionKindFindings, true
	case AnalysisAnswer:
		return SolutionKindAnswer, true
	}
	return "", false
}

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

	// Artifact columns (CR-REQ-027). Seq stays 0 until MintSolutionID assigns it.
	Seq                  int
	SchemaVersion        int
	ProvenanceJSON       []byte
	InputRequestRevision int
	ContentDigest        string
}

// StampArtifact records how and from which Request revision the document was produced (CR-REQ-027).
func (s *Solution) StampArtifact(provenance []byte, digest string, requestRevision int) {
	s.ProvenanceJSON, s.ContentDigest, s.InputRequestRevision = provenance, digest, requestRevision
	if s.SchemaVersion == 0 {
		s.SchemaVersion = 1
	}
}

// Propose fills a draft with the validated document.
func (s *Solution) Propose(opts []byte) error {
	if s.Status != SolutionStatusDraft {
		return ErrSolutionNotDraft
	}
	s.OptionsJSON = opts
	s.Status = SolutionStatusProposed
	return nil
}

// Choose picks an option of a proposed solution; the same index again is accepted.
func (s *Solution) Choose(idx, optionCount int) error {
	if s.Status != SolutionStatusProposed {
		return ErrSolutionNotProposedState
	}
	if idx < 0 || idx >= optionCount {
		return ErrOptionIndexOutOfRange
	}
	s.ChosenOption = &idx
	return nil
}

// Supersede retires an unapproved solution; superseding twice is a no-op so redelivery is safe.
func (s *Solution) Supersede() error {
	switch s.Status {
	case SolutionStatusSuperseded:
		return nil
	case SolutionStatusApproved:
		return ErrSolutionNotSupersedable
	}
	s.Status = SolutionStatusSuperseded
	return nil
}

func (s *Solution) Approve() error {
	if s.Status != SolutionStatusProposed {
		return ErrSolutionNotProposedState
	}
	s.Status = SolutionStatusApproved
	return nil
}

// AutoApprove approves a draft that has no human gate (hotfix diagnosis).
func (s *Solution) AutoApprove(opts []byte) error {
	if err := s.Propose(opts); err != nil {
		return err
	}
	return s.Approve()
}

func (s *Solution) Reject() error {
	if s.Status != SolutionStatusProposed {
		return ErrSolutionNotProposedState
	}
	s.Status = SolutionStatusRejected
	s.ChosenOption = nil // a rejected choice must not leak into planning
	return nil
}
