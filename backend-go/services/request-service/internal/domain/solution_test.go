package domain

import "testing"

func TestSolution_Lifecycle(t *testing.T) {
	s := Solution{Kind: SolutionKindSolution, Status: SolutionStatusDraft}
	if err := s.Choose(0, 2); err != ErrSolutionNotProposedState {
		t.Fatalf("choose on draft = %v", err)
	}
	if err := s.Propose([]byte(`{}`)); err != nil || s.Status != SolutionStatusProposed {
		t.Fatalf("propose = %v %v", err, s.Status)
	}
	if err := s.Propose([]byte(`{}`)); err != ErrSolutionNotDraft {
		t.Fatalf("second propose = %v", err)
	}
	if err := s.Choose(2, 2); err != ErrOptionIndexOutOfRange {
		t.Fatalf("out of range = %v", err)
	}
	if err := s.Choose(-1, 2); err != ErrOptionIndexOutOfRange {
		t.Fatalf("negative = %v", err)
	}
	if err := s.Choose(1, 2); err != nil || s.ChosenOption == nil || *s.ChosenOption != 1 {
		t.Fatalf("choose = %v %v", err, s.ChosenOption)
	}
	if err := s.Approve(); err != nil || s.Status != SolutionStatusApproved {
		t.Fatalf("approve = %v", err)
	}
	if err := s.Supersede(); err != ErrSolutionNotSupersedable {
		t.Fatalf("supersede approved = %v", err)
	}
	if err := s.Reject(); err != ErrSolutionNotProposedState {
		t.Fatalf("reject approved = %v", err)
	}
}

func TestSolution_RejectClearsChoice(t *testing.T) {
	one := 1
	s := Solution{Status: SolutionStatusProposed, ChosenOption: &one}
	if err := s.Reject(); err != nil || s.Status != SolutionStatusRejected || s.ChosenOption != nil {
		t.Fatalf("reject = %v %+v", err, s)
	}
	if err := s.Supersede(); err != nil || s.Status != SolutionStatusSuperseded {
		t.Fatalf("supersede rejected = %v", err)
	}
	if err := s.Supersede(); err != nil {
		t.Fatalf("second supersede must be a no-op, got %v", err)
	}
}

func TestSolution_AutoApprove(t *testing.T) {
	s := Solution{Status: SolutionStatusDraft}
	if err := s.AutoApprove([]byte(`{"a":1}`)); err != nil || s.Status != SolutionStatusApproved || string(s.OptionsJSON) != `{"a":1}` {
		t.Fatalf("%v %+v", err, s)
	}
	if err := s.AutoApprove(nil); err == nil {
		t.Fatal("auto-approving a non-draft must fail")
	}
}

func TestKindForAnalysis(t *testing.T) {
	for in, want := range map[AnalysisKind]SolutionKind{AnalysisSolution: SolutionKindSolution, AnalysisDiagnosis: SolutionKindDiagnosis, AnalysisFindings: SolutionKindFindings, AnalysisAnswer: SolutionKindAnswer} {
		if got, ok := KindForAnalysis(in); !ok || got != want {
			t.Errorf("%s -> %s %v", in, got, ok)
		}
	}
	if _, ok := KindForAnalysis(AnalysisNone); ok {
		t.Error("none has no solution kind")
	}
}
