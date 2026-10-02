package domain

import (
	"strings"
	"testing"
	"time"
)

func TestToolCallAuditEventCarriesTraceID(t *testing.T) {
	c := ToolCall{ID: "c", TenantID: "t", UserID: "u", ToolName: "git_status", Decision: CallAllow, TraceID: "0af7651916cd43dd8448eb211c80319c"}
	ev, err := NewToolCallAuditEvent("e1", c, time.Unix(0, 0), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ev.PayloadJSON), `"trace_id":"0af7651916cd43dd8448eb211c80319c"`) {
		t.Fatalf("trace_id missing: %s", ev.PayloadJSON)
	}
	c.TraceID = ""
	ev, _ = NewToolCallAuditEvent("e2", c, time.Unix(0, 0), 0)
	if strings.Contains(string(ev.PayloadJSON), "trace_id") {
		t.Fatalf("untraced call must not emit an empty trace_id: %s", ev.PayloadJSON)
	}
}
