package eventbus

import "testing"

// TestSubjects_IncludesProjectDevServerChanged is a regression guard for
// TASK-PRF-03-08's cross-service wiring: a missing entry here fails
// silently (the event is simply never consumed, no error anywhere), so
// this asserts the static subscription list directly rather than relying
// on an integration test to notice.
func TestSubjects_IncludesProjectDevServerChanged(t *testing.T) {
	want := SubjectBinding{StreamName: "PROJECT", Subject: "orca.project.devserver.changed"}
	for _, b := range Subjects {
		if b == want {
			return
		}
	}
	t.Errorf("expected %+v in Subjects, got %+v", want, Subjects)
}

func TestSubjects_IncludesMcpApprovalRequested(t *testing.T) {
	for _, b := range Subjects {
		if b.StreamName == "MCP" && b.Subject == "orca.mcp.approval.requested" {
			return
		}
	}
	t.Fatal("notification-service must consume orca.mcp.approval.requested from the MCP stream")
}

func TestSubjects_McpTerminalIdleStoppedIsDurableOnTheMcpStream(t *testing.T) {
	for _, b := range Subjects {
		if b.Subject == "orca.mcp.terminal.idlestopped" {
			if b.StreamName != "MCP" || b.Durable != "notification-service-mcp-terminal-idle-stopped" {
				t.Fatalf("binding = %+v", b)
			}
			return
		}
	}
	t.Fatal("idle-stop subject is not consumed")
}

func TestSubjects_InfraFleetTerminalClosedIsDurableAndLegacyBindingRemains(t *testing.T) {
	var closed, legacy bool
	for _, b := range Subjects {
		switch b.Subject {
		case "orca.infrafleet.terminal.closed":
			closed = b.StreamName == "INFRAFLEET" && b.Durable == "notification-service-infrafleet-terminal-closed"
		case "orca.mcp.terminal.idlestopped":
			legacy = true
		}
	}
	if !closed || !legacy {
		t.Fatalf("closed binding ok=%v legacy binding present=%v", closed, legacy)
	}
}
