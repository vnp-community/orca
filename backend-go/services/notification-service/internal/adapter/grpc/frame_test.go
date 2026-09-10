package grpc

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func TestFramePayloadJSON_IncludesIsRead(t *testing.T) {
	e := domain.NotificationEvent{Title: "t", Body: "b", Severity: domain.SeverityInfo, IsRead: true}
	got := framePayloadJSON(e)
	if !strings.Contains(got, `"is_read":true`) {
		t.Fatalf("expected payload to contain is_read:true, got %s", got)
	}
}

func TestFramePayloadJSON_DefaultsIsReadFalse(t *testing.T) {
	// A freshly translated event (TranslateEvent never sets IsRead) must
	// serialize as unread — regression guard for the new field.
	e := domain.NotificationEvent{Title: "t", Body: "b", Severity: domain.SeverityInfo}
	got := framePayloadJSON(e)
	if !strings.Contains(got, `"is_read":false`) {
		t.Fatalf("expected payload to contain is_read:false, got %s", got)
	}
}
