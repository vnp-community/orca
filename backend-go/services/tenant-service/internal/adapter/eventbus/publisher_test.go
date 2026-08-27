package eventbus

import (
	"encoding/json"
	"testing"
)

// TestAuditPayload_MarshalsExpectedShape guards against a JSON-tag typo
// (e.g. actorId vs actor_id) in auditPayload — PublishAuditEvent's actual
// network call requires a live NATS/JetStream connection this unit test
// suite doesn't stand up, so this asserts the one thing that fails
// silently downstream: the wire shape of the payload it marshals.
func TestAuditPayload_MarshalsExpectedShape(t *testing.T) {
	payload := auditPayload{Action: "company.profile.updated", ActorID: "u1", Target: "c1"}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]any{"action": "company.profile.updated", "actor_id": "u1", "target": "c1"}
	for k, v := range want {
		if decoded[k] != v {
			t.Errorf("field %q: got %v, want %v (raw json: %s)", k, decoded[k], v, b)
		}
	}
	if len(decoded) != len(want) {
		t.Errorf("unexpected field count: got %v, want keys %v", decoded, want)
	}
}

func TestAuditSubject_Value(t *testing.T) {
	if AuditSubject != "orca.tenant.audit.recorded" {
		t.Errorf("AuditSubject = %q, want %q", AuditSubject, "orca.tenant.audit.recorded")
	}
}
