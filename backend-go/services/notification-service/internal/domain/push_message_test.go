package domain

import "testing"

func TestToPushMessage_SanitizesDeepLink(t *testing.T) {
	for in, want := range map[string]string{"/ok?a=1": "/ok?a=1", "//evil.com": "", "https://evil.com": "", "/\\evil": "", "": ""} {
		if got := ToPushMessage(NotificationEvent{DeepLink: in}).DeepLink; got != want {
			t.Errorf("deepLink %q -> %q, want %q", in, got, want)
		}
	}
}

func TestToPushMessage_MCPApprovalBodyIsFixed(t *testing.T) {
	m := ToPushMessage(NotificationEvent{ID: "1", Type: MCPApprovalType, Body: "tool {\"secret\":1}"})
	if m.Body != mcpApprovalPushBody || m.Tag != "mcp.approval:1" {
		t.Errorf("%+v", m)
	}
}

func TestPushDeliveryFor(t *testing.T) {
	if d := PushDeliveryFor(NotificationEvent{Severity: SeverityCritical}); d.Urgency != "high" {
		t.Errorf("%+v", d)
	}
	if d := PushDeliveryFor(NotificationEvent{Severity: SeverityInfo}); d.Urgency != "normal" {
		t.Errorf("%+v", d)
	}
}
