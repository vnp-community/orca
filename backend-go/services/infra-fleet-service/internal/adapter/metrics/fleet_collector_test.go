package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// collectAll runs Collect and returns every emitted metric — a minimal
// stand-in for prometheus/client_golang/prometheus/testutil.CollectAndCompare
// so this package doesn't need to add that test-only dependency.
func collectAll(t *testing.T, c prometheus.Collector) []prometheus.Metric {
	t.Helper()
	ch := make(chan prometheus.Metric, 64)
	go func() {
		c.Collect(ch)
		close(ch)
	}()
	var out []prometheus.Metric
	for m := range ch {
		out = append(out, m)
	}
	return out
}

func metricName(t *testing.T, m prometheus.Metric) string {
	t.Helper()
	var pb dto.Metric
	if err := m.Write(&pb); err != nil {
		t.Fatalf("writing metric: %v", err)
	}
	return m.Desc().String()
}

func TestFleetCollector_EmitsFourNamedMetricsWithServerLabel(t *testing.T) {
	c := NewFleetCollector()
	c.Update("ds1", "dev1.example.com", domain.DevServerHealth{
		DevServerID: "ds1", Reachable: true, CPUPercent: 42, RAMPercent: 55, DiskPercent: 10,
		LatencyMS: 123, Status: domain.HealthStatusHealthy,
	})

	metrics := collectAll(t, c)
	if len(metrics) != 4 {
		t.Fatalf("expected exactly 4 metrics for 1 dev server, got %d", len(metrics))
	}

	wantNames := []string{"orca_fleet_server_status", "orca_fleet_cpu_percent", "orca_fleet_ram_percent", "orca_fleet_ssh_latency_ms"}
	for _, want := range wantNames {
		found := false
		for _, m := range metrics {
			desc := metricName(t, m)
			if strings.Contains(desc, `fqName: "`+want+`"`) {
				found = true
				var pb dto.Metric
				if err := m.Write(&pb); err != nil {
					t.Fatalf("writing metric %s: %v", want, err)
				}
				var labeled bool
				for _, l := range pb.GetLabel() {
					if l.GetName() == "server" && l.GetValue() == "dev1.example.com" {
						labeled = true
					}
				}
				if !labeled {
					t.Errorf("expected metric %s to carry server=dev1.example.com, got labels %+v", want, pb.GetLabel())
				}
			}
		}
		if !found {
			t.Errorf("expected a metric named %q, got descs: %v", want, describeAll(t, metrics))
		}
	}
}

func describeAll(t *testing.T, metrics []prometheus.Metric) []string {
	t.Helper()
	out := make([]string, len(metrics))
	for i, m := range metrics {
		out[i] = m.Desc().String()
	}
	return out
}

func TestFleetCollector_HealthyAsFloatIsOneOnlyWhenHealthy(t *testing.T) {
	tests := []struct {
		status domain.HealthStatus
		want   float64
	}{
		{domain.HealthStatusHealthy, 1},
		{domain.HealthStatusDegraded, 0},
		{domain.HealthStatusUnhealthy, 0},
		{domain.HealthStatusUnreachable, 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := healthyAsFloat(tt.status); got != tt.want {
				t.Errorf("healthyAsFloat(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestFleetCollector_MultipleServersEmitIndependentLabeledMetrics(t *testing.T) {
	c := NewFleetCollector()
	c.Update("ds1", "dev1.example.com", domain.DevServerHealth{Status: domain.HealthStatusHealthy, CPUPercent: 10})
	c.Update("ds2", "dev2.example.com", domain.DevServerHealth{Status: domain.HealthStatusDegraded, CPUPercent: 90})

	metrics := collectAll(t, c)
	if len(metrics) != 8 { // 4 metrics x 2 servers
		t.Fatalf("expected 8 metrics for 2 dev servers, got %d", len(metrics))
	}

	seenServers := map[string]bool{}
	for _, m := range metrics {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			t.Fatalf("writing metric: %v", err)
		}
		for _, l := range pb.GetLabel() {
			if l.GetName() == "server" {
				seenServers[l.GetValue()] = true
			}
		}
	}
	if !seenServers["dev1.example.com"] || !seenServers["dev2.example.com"] {
		t.Errorf("expected metrics labeled for both servers, got %+v", seenServers)
	}
}

func TestFleetCollector_DescribeSendsFourDescs(t *testing.T) {
	c := NewFleetCollector()
	ch := make(chan *prometheus.Desc, 64)
	go func() {
		c.Describe(ch)
		close(ch)
	}()
	var count int
	for range ch {
		count++
	}
	if count != 4 {
		t.Errorf("expected Describe to send 4 descs, got %d", count)
	}
}
