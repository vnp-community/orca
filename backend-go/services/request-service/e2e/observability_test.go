//go:build e2e

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", theStack.httpPort))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// /metrics of the running service serves the Request series, with the database gauges sampled, and no id-like label.
func TestMetricsEndpointServesRequestSeries(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.classified("metrics", "bug", "M")
	k.confirm(r, "bug", "M")

	eventually(t, 20*time.Second, "the Request series on /metrics", func() (bool, string) {
		out := scrapeMetrics(t)
		for _, want := range []string{
			`orca_request_created_total{source_provider="manual"}`,
			`orca_request_transitions_total{from="awaiting_type_confirmation",to="analyzing",type="bug"}`,
			`orca_request_classification_total{outcome="confirmed_as_proposed"}`,
			`orca_request_classification_confidence_count`,
			`orca_request_ai_generation_seconds_count{kind="classification",outcome="ok"}`,
			`orca_request_outbox_pending `,
			`orca_request_stuck{status="classifying"}`,
		} {
			if !strings.Contains(out, want) {
				return false, "missing " + want
			}
		}
		return true, ""
	})
	out := scrapeMetrics(t)
	if m := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).FindString(out); m != "" {
		t.Fatalf("an id (%s) became a metric label", m)
	}
	if strings.Contains(out, "metrics e2e") {
		t.Fatal("a Request title reached /metrics")
	}
}

// Audit entries for a real flow reach auth-service with the right actor, and never carry the title or body.
func TestAuditTrailOfARealFlow(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.classified("audit trail", "bug", "M")
	k.confirm(r, "bug", "M")
	if _, err := k.req.ChangeRequestType(k.asReporter(), changeTypeReq(r.GetId(), "change_request")); err != nil {
		t.Fatal(err)
	}

	eventually(t, 15*time.Second, "audit entries", func() (bool, string) {
		a := k.audit()
		return hasAction(a, domain.ActionRequestCreate, "allowed") && hasAction(a, domain.ActionRequestTypeConfirm, "allowed") &&
			hasAction(a, domain.ActionRequestTypeChange, "allowed") && hasAction(a, domain.ActionApprovalApprove, "allowed"), fmt.Sprintf("%+v", a)
	})
	for _, e := range k.audit() {
		if e.TargetType == "" || e.TargetID == "" || e.Target != e.TargetType+":"+e.TargetID {
			t.Errorf("target not in the type:id form: %+v", e)
		}
		if e.ActorType != "user" && e.ActorType != "agent" && e.ActorType != "system" {
			t.Errorf("actor type %q is not allowed by auth.audit_log: %+v", e.ActorType, e)
		}
		if strings.Contains(e.MetadataJSON, "audit trail") || strings.Contains(e.MetadataJSON, "e2e body") {
			t.Errorf("metadata leaks title or body: %s", e.MetadataJSON)
		}
	}
}
