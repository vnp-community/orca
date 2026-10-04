package tracing

// Trace spans live under orca.trace.> rather than orca.<service>.trace.span:
// that wildcard overlapped every orca.<service>.> domain stream, and JetStream
// rejects overlapping streams, so whichever service started first decided
// whether the TASK/WORKFLOW/MCP/... streams could be created at all.
const (
	TraceStreamName     = "TRACE"
	TraceStreamSubjects = "orca.trace.>"
	// TraceSubscribeSubject matches every service's spans.
	TraceSubscribeSubject = "orca.trace.*.span"
)

// TraceSubject is the subject a service publishes its spans on.
func TraceSubject(serviceName string) string {
	return "orca.trace." + serviceName + ".span"
}
