package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type reporterOfRequest struct {
	reporter string
	err      error
}

func (g reporterOfRequest) Get(context.Context, string) (domain.Request, error) {
	return domain.Request{ReporterID: g.reporter}, g.err
}

func TestAnalysisRunner_PlanRunResolvesAsReporter(t *testing.T) {
	base := tenant.WithUserID(context.Background(), "approver")
	r := &AnalysisRunner{requests: reporterOfRequest{reporter: "rep"}}
	run := domain.AnalysisRun{Kind: domain.RunKindPlan, ActorID: "approver"}
	if got, _ := tenant.UserID(r.reporterIdentity(base, run)); got != "rep" {
		t.Fatalf("a plan run must resolve as the reporter, got %q", got)
	}
	if run.ActorID != "approver" {
		t.Fatal("the run stays attributed to the approver")
	}
}

func TestAnalysisRunner_ReporterIdentityForNonReporterActors(t *testing.T) {
	base := tenant.WithUserID(context.Background(), "approver")
	cases := []struct {
		name string
		r    *AnalysisRunner
		run  domain.AnalysisRun
		want string
	}{
		{"approver's analysis run looks up the dev server as the reporter", &AnalysisRunner{requests: reporterOfRequest{reporter: "rep"}}, domain.AnalysisRun{Kind: domain.RunKindSolution, ActorID: "approver"}, "rep"},
		{"reporter's own run is untouched", &AnalysisRunner{requests: reporterOfRequest{reporter: "approver"}}, domain.AnalysisRun{Kind: domain.RunKindSolution, ActorID: "approver"}, "approver"},
		{"an approver's plan run looks up the dev server as the reporter", &AnalysisRunner{requests: reporterOfRequest{reporter: "rep"}}, domain.AnalysisRun{Kind: domain.RunKindPlan, ActorID: "approver"}, "rep"},
		{"the reporter's own plan run is untouched", &AnalysisRunner{requests: reporterOfRequest{reporter: "approver"}}, domain.AnalysisRun{Kind: domain.RunKindPlan, ActorID: "approver"}, "approver"},
		{"a failed lookup keeps the caller of a plan run", &AnalysisRunner{requests: reporterOfRequest{err: errors.New("down")}}, domain.AnalysisRun{Kind: domain.RunKindPlan, ActorID: "approver"}, "approver"},
		{"a failed lookup keeps the caller", &AnalysisRunner{requests: reporterOfRequest{err: errors.New("down")}}, domain.AnalysisRun{Kind: domain.RunKindSolution, ActorID: "approver"}, "approver"},
		{"no store wired keeps the caller", &AnalysisRunner{}, domain.AnalysisRun{Kind: domain.RunKindSolution, ActorID: "approver"}, "approver"},
	}
	for _, c := range cases {
		got, _ := tenant.UserID(c.r.reporterIdentity(base, c.run))
		if got != c.want {
			t.Errorf("%s: user = %q, want %q", c.name, got, c.want)
		}
	}
}
