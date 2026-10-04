package tracing

import (
	"strings"
	"testing"
)

func TestTraceSubjectFitsStream(t *testing.T) {
	got := TraceSubject("task-service")
	if got != "orca.trace.task-service.span" {
		t.Fatalf("TraceSubject = %q", got)
	}
	if !strings.HasPrefix(got, strings.TrimSuffix(TraceStreamSubjects, ">")) {
		t.Fatalf("%q is outside stream subjects %q", got, TraceStreamSubjects)
	}
}

// Guards the startup race: JetStream rejects overlapping streams, so the trace
// stream must not capture subjects owned by a domain stream (orca.<svc>.>).
func TestTraceStreamDoesNotOverlapDomainStreams(t *testing.T) {
	for _, svc := range []string{"task", "workflow", "mcp", "tenant", "infrafleet", "orchestration"} {
		if strings.HasPrefix("orca."+svc+".", strings.TrimSuffix(TraceStreamSubjects, ">")) {
			t.Fatalf("trace stream %q overlaps orca.%s.>", TraceStreamSubjects, svc)
		}
	}
}
