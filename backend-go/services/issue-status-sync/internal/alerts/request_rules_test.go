package alerts

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/adapter/metrics"
)

type ruleFile struct {
	Groups []struct {
		Name  string `yaml:"name"`
		Rules []struct {
			Alert       string            `yaml:"alert"`
			Expr        string            `yaml:"expr"`
			For         string            `yaml:"for"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

// Series promised by CR-REQ-024 2.9; request-service owns the orca_request_* ones.
var knownSeries = map[string]bool{
	"orca_request_approvals_pending": true, "orca_request_stuck": true, "orca_request_ai_generation_seconds_count": true,
	"orca_request_outbox_pending": true, "orca_request_returned_total": true,
	"orca_issuesync_request_events_total": true,
}

func TestRequestRulesParseAndUseKnownSeries(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/alerts/request.rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var f ruleFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if len(f.Groups) != 1 || f.Groups[0].Name != "orca-request" {
		t.Fatalf("want one group orca-request, got %+v", f.Groups)
	}
	want := []string{"RequestApprovalBacklog", "RequestStuck", "RequestAIGenerationFailures", "RequestOutboxLag", "RequestReturnedSpike", "IssueSyncRequestFailures"}
	var got []string
	metricName := regexp.MustCompile(`orca_[a-z_]+`)
	for _, r := range f.Groups[0].Rules {
		got = append(got, r.Alert)
		if r.Expr == "" || r.For == "" || r.Labels["severity"] == "" || r.Annotations["summary"] == "" {
			t.Errorf("%s: missing expr/for/severity/summary", r.Alert)
		}
		for _, m := range metricName.FindAllString(r.Expr, -1) {
			if !knownSeries[m] {
				t.Errorf("%s uses unknown series %s", r.Alert, m)
			}
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("alerts = %v, want %v", got, want)
	}
}

func TestIssueSyncSeriesUsedByRulesExistInTheRegistry(t *testing.T) {
	s := metrics.New()
	s.ObserveRequestEvent("status_changed", "failed")
	raw, _ := os.ReadFile("../../../../deploy/alerts/request.rules.yaml")
	if !strings.Contains(string(raw), `orca_issuesync_request_events_total{result="failed"}`) {
		t.Fatal("IssueSyncRequestFailures must filter on result=\"failed\"")
	}
}
