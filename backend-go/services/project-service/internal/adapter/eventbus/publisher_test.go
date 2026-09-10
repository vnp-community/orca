package eventbus

import (
	"encoding/json"
	"testing"
)

// TestAuditPayload_MarshalsExpectedShape and
// TestNotificationEventPayload_MarshalsExpectedShape guard against a
// JSON-tag typo (e.g. userIds vs user_ids — the exact concern this task's
// spec flags) in the two payload structs. PublishAuditEvent/
// NotifyDevServerChanged's actual network call requires a live
// NATS/JetStream connection this unit test suite doesn't stand up, so this
// asserts the one thing that fails silently downstream: the wire shape.
func TestAuditPayload_MarshalsExpectedShape(t *testing.T) {
	payload := auditPayload{Action: "project.devserver.changed", ActorID: "u1", Target: "p1"}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{"action": "project.devserver.changed", "actor_id": "u1", "target": "p1"}
	for k, v := range want {
		if decoded[k] != v {
			t.Errorf("field %q: got %v, want %v (raw json: %s)", k, decoded[k], v, b)
		}
	}
	if len(decoded) != len(want) {
		t.Errorf("unexpected field count: got %v, want keys %v", decoded, want)
	}
}

func TestNotificationEventPayload_MarshalsExpectedShape(t *testing.T) {
	payload := notificationEventPayload{
		UserIDs: []string{"u1", "u2"}, Title: "Dev server changed",
		Body: "body", DeepLink: "/projects/p1",
	}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["user_ids"]; !ok {
		t.Errorf("expected key %q, got %v", "user_ids", decoded)
	}
	if _, ok := decoded["deep_link"]; !ok {
		t.Errorf("expected key %q, got %v", "deep_link", decoded)
	}
	if decoded["title"] != "Dev server changed" {
		t.Errorf("unexpected title: %v", decoded["title"])
	}
}

func TestSubjects_Values(t *testing.T) {
	if AuditSubject != "orca.project.audit.recorded" {
		t.Errorf("AuditSubject = %q", AuditSubject)
	}
	if DevServerChangedSubject != "orca.project.devserver.changed" {
		t.Errorf("DevServerChangedSubject = %q", DevServerChangedSubject)
	}
}
