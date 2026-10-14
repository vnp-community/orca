package usecase

import "testing"

func TestAnalysisSettings_SpecBudgets(t *testing.T) {
	d := AnalysisSettings{}.withDefaults()
	if d.SpecTimeoutMS != 900_000 || d.SpecTotalTimeoutMS != 2*900_000+120_000 {
		t.Fatalf("defaults: %+v", d)
	}
	short := AnalysisSettings{SpecTimeoutMS: 300_000}.withDefaults()
	if short.SpecTotalTimeoutMS != 2*300_000+120_000 {
		t.Fatalf("a shorter attempt budget shrinks the total with it: %d", short.SpecTotalTimeoutMS)
	}
	pinned := AnalysisSettings{SpecTimeoutMS: 300_000, SpecTotalTimeoutMS: 400_000}.withDefaults()
	if pinned.SpecTotalTimeoutMS != 400_000 {
		t.Fatalf("an operator's total wins: %d", pinned.SpecTotalTimeoutMS)
	}
	if d.AICompleteTimeout.Seconds() != 120 || d.AgentTimeoutMS != 600_000 {
		t.Fatalf("native values are unchanged: %+v", d)
	}
}
