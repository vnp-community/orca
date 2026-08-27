package grpc

import (
	"encoding/json"
	"log/slog"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// framePayload is the JSON shape encoded into
// NotificationServiceStreamNotificationsResponse.PayloadJson — the proto
// keeps this untyped (payload_json: string) so a new NotificationEvent
// field doesn't require a proto/schema change to reach the client, per
// notification-service.md §3's "illustrative, not exhaustive" subject list.
type framePayload struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link,omitempty"`
	Severity string `json:"severity"`
}

// framePayloadJSON marshals e's user-facing fields to JSON for the wire
// frame. Marshaling a fixed, known-safe struct cannot fail; a failure here
// would indicate a bug in this function, not bad input, so it degrades to
// an empty object rather than dropping the frame's id/type fields too.
func framePayloadJSON(e domain.NotificationEvent) string {
	b, err := json.Marshal(framePayload{
		Title:    e.Title,
		Body:     e.Body,
		DeepLink: e.DeepLink,
		Severity: string(e.Severity),
	})
	if err != nil {
		slog.Warn("failed to marshal notification frame payload", slog.Any("error", err))
		return "{}"
	}
	return string(b)
}
