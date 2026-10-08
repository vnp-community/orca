package eventbus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type recordingReporter struct {
	got []usecase.ReportTaskOutcomeInput
	err error
	tid []string
}

func (r *recordingReporter) Execute(ctx context.Context, in usecase.ReportTaskOutcomeInput) error {
	r.got = append(r.got, in)
	id, _ := tenant.TenantID(ctx)
	r.tid = append(r.tid, id)
	return r.err
}

func fixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The fixtures are the payloads task-service emits (copied from its own tests); they must classify as the CR table says.
func TestTaskOutcomeConsumer_DecodesRealPayload(t *testing.T) {
	cases := []struct {
		file string
		want domain.Outcome
	}{
		{"statuschanged_execute_claim.json", domain.OutcomeStarted},
		{"statuschanged_execution_completed.json", domain.OutcomeSucceeded},
		{"statuschanged_user_done.json", domain.OutcomeSucceeded},
		{"statuschanged_execution_failed.json", domain.OutcomeFailed},
		{"statuschanged_recovery.json", domain.OutcomeFailed},
		{"statuschanged_phase_derived_done.json", domain.OutcomePhaseDone},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			rep := &recordingReporter{}
			c := &TaskOutcomeConsumer{reporter: rep}
			at := time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC)
			err := c.Handle(context.Background(), commoneventbus.Event{ID: "evt-1", TenantID: "tenant-1", OccurredAt: at, Payload: fixture(t, tc.file)})
			if err != nil || len(rep.got) != 1 {
				t.Fatalf("got %v %v", rep.got, err)
			}
			in := rep.got[0]
			if in.EventID != "evt-1" || in.RequestID != "req-1" || rep.tid[0] != "tenant-1" || !in.OccurredAt.Equal(at) {
				t.Fatalf("envelope fields: %+v tenant=%v", in, rep.tid)
			}
			got, ok := domain.ClassifyTaskOutcome(in.TaskType, in.Cause, in.NewStatus)
			if !ok || got != tc.want {
				t.Fatalf("classified %q (%v), want %q", got, ok, tc.want)
			}
		})
	}
	rep := &recordingReporter{}
	c := &TaskOutcomeConsumer{reporter: rep}
	_ = c.Handle(context.Background(), commoneventbus.Event{ID: "e", TenantID: "t", Payload: fixture(t, "statuschanged_execution_failed.json")})
	if in := rep.got[0]; in.ErrorMessage != "agent crashed" || in.ExecutionLinkID != "link-1" || in.ParentID != "phase-1" || in.TaskID != "task-1" {
		t.Fatalf("failure details lost: %+v", in)
	}
}

func TestTaskOutcomeConsumer_IgnoresWhatIsNotARequestTask(t *testing.T) {
	rep := &recordingReporter{}
	c := &TaskOutcomeConsumer{reporter: rep}
	for name, ev := range map[string]commoneventbus.Event{
		"task without request": {ID: "1", TenantID: "t", Payload: fixture(t, "statuschanged_plain_task.json")},
		"garbage":              {ID: "2", TenantID: "t", Payload: json.RawMessage(`not json`)},
		"no tenant":            {ID: "3", Payload: fixture(t, "statuschanged_execution_failed.json")},
	} {
		if err := c.Handle(context.Background(), ev); err != nil {
			t.Errorf("%s: must be acked (nil), got %v", name, err)
		}
	}
	if len(rep.got) != 0 {
		t.Fatalf("nothing should reach the use case: %+v", rep.got)
	}
}

func TestTaskOutcomeConsumer_ErrorsAreRedelivered(t *testing.T) {
	rep := &recordingReporter{err: context.DeadlineExceeded}
	c := &TaskOutcomeConsumer{reporter: rep}
	if err := c.Handle(context.Background(), commoneventbus.Event{ID: "1", TenantID: "t", Payload: fixture(t, "statuschanged_execution_failed.json")}); err == nil {
		t.Fatal("a failed write must be returned so JetStream redelivers")
	}
}

// The NATS integration test reads these from its own goroutine while the consumer writes them.
type recordingStarter struct {
	mu  sync.Mutex
	ids []string
	err error
}

func (s *recordingStarter) Execute(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids = append(s.ids, id)
	return true, s.err
}

func (s *recordingStarter) started() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...)
}

func executionStatusEvent(to string) commoneventbus.Event {
	p, _ := json.Marshal(map[string]string{"request_id": "req-1", "to": to, "trigger": "plan_approved"})
	return commoneventbus.Event{ID: "evt", TenantID: "t1", Payload: p}
}

func TestRequestStatusConsumer_OnlyExecutingStarts(t *testing.T) {
	s := &recordingStarter{}
	c := &RequestStatusConsumer{starter: s}
	for _, to := range []string{"classifying", "analyzing", "request_backlog", "completed", "executing"} {
		if err := c.Handle(context.Background(), executionStatusEvent(to)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.ids) != 1 || s.ids[0] != "req-1" {
		t.Fatalf("only the executing event starts: %v", s.ids)
	}
	bad := commoneventbus.Event{ID: "x", TenantID: "t", Payload: json.RawMessage(`{`)}
	if err := c.Handle(context.Background(), bad); err != nil {
		t.Fatalf("garbage is acked: %v", err)
	}
	s.err = context.DeadlineExceeded
	if err := c.Handle(context.Background(), executionStatusEvent("executing")); err == nil {
		t.Fatal("a failing start is retried")
	}
}

type recordingResumer struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingResumer) Execute(_ context.Context, id string, st domain.SubjectType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, id+"/"+string(st))
	return nil
}

func (r *recordingResumer) resumed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func TestApprovalDecidedConsumer_OnlyApprovedPreDeployResumes(t *testing.T) {
	r := &recordingResumer{}
	c := &ApprovalDecidedConsumer{resumer: r}
	send := func(subject, decision string) {
		p, _ := json.Marshal(domain.ApprovalDecidedPayload{RequestID: "req-1", SubjectType: subject, Decision: decision})
		if err := c.Handle(context.Background(), commoneventbus.Event{ID: "e", TenantID: "t", Payload: p}); err != nil {
			t.Fatal(err)
		}
	}
	send("phase", "approved")
	send("pre_deploy", "rejected")
	send("pre_deploy", "expired")
	send("pre_deploy", "approved")
	if len(r.calls) != 1 || r.calls[0] != "req-1/pre_deploy" {
		t.Fatalf("got %v", r.calls)
	}
}
