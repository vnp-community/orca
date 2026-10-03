package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSet_ExposesDeliveryCounters(t *testing.T) {
	s := New()
	s.ObserveDelivery("web", "sent", 10*time.Millisecond)
	s.ObserveDelivery("web", "expired", 10*time.Millisecond)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{`orca_notification_push_deliveries_total{channel="web",outcome="sent"} 1`, `outcome="expired"`, "orca_notification_push_delivery_seconds_count"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}
