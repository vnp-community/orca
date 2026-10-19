package contracttest

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// flowBaseRefOnBothPaths runs three Requests through the real loop and checks the base ref task-service receives: the plain
// run and the contract run (OpenSpec) both carry it, an unset ref goes out empty, and a ref set after the Request was
// created is read from the stored Request at dispatch time (CR-REQ-039).
func flowBaseRefOnBothPaths(t *testing.T, env ExecutionEnv) {
	writer, ok := env.Requests.(usecase.RequestBaseRefWriter)
	if !ok {
		t.Fatal("the request repository must store base refs")
	}
	cases := []struct {
		name     string
		engine   domain.EngineName
		seedRef  string // stored when the Request is created
		laterRef string // set through the repository after that, before execution starts
		want     string
		contract bool
	}{
		{name: "plain run with a ref", engine: domain.EngineNative, seedRef: "release/x", want: "release/x"},
		{name: "contract run with a ref", engine: domain.EngineOpenSpec, seedRef: "release/x", want: "release/x", contract: true},
		{name: "contract run, ref set after creation", engine: domain.EngineOpenSpec, laterRef: "v2.0", want: "v2.0", contract: true},
		{name: "plain run, no ref", engine: domain.EngineNative, want: ""},
		{name: "contract run, no ref", engine: domain.EngineOpenSpec, want: "", contract: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFlowRig(t, env)
			r.advance.Contract = usecase.OpenSpecContractRuns{}
			engine := tc.engine
			req := seedExecuting(t, env, r.ctx, func(q *domain.Request) {
				q.SolutionEngine, q.BaseRef = &engine, tc.seedRef
			})
			if tc.laterRef != "" {
				if _, err := writer.SetBaseRef(r.ctx, req.ID, tc.laterRef, req.Version); err != nil {
					t.Fatal(err)
				}
			}
			plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
			phase := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
			first := r.tasks.Add(req.ID, "task", "t1", phase, "open")
			second := r.tasks.Add(req.ID, "task", "t2", phase, "blocked")
			r.tasks.DependsOn(second, first)
			approver := uuid.NewString()
			r.tasks.AllowTreeReaders(approver)
			r.approve(req, domain.SubjectPlan, plan, approver)
			r.pinContinuous(req)

			if started, err := r.startExec.Execute(r.ctx, req.ID); err != nil || !started {
				t.Fatalf("StartExecution: started=%v %v", started, err)
			}
			r.settle()
			r.tasks.RunSucceeded(first)
			r.settle()

			calls := r.tasks.ExecuteCalls()
			if len(calls) != 2 {
				t.Fatalf("both tasks of the Plan are dispatched: %+v", calls)
			}
			for _, c := range calls {
				if c.BaseRef != tc.want || c.Contract != tc.contract {
					t.Fatalf("dispatch %+v, want base ref %q contract=%v", c, tc.want, tc.contract)
				}
			}
			if got := []string{calls[0].TaskID, calls[1].TaskID}; !slices.Equal(got, []string{first, second}) {
				t.Fatalf("dispatch order = %v", got)
			}
		})
	}
}
