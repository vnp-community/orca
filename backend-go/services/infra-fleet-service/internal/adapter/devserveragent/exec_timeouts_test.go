package devserveragent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestExecTimeoutForMethod_Table(t *testing.T) {
	tests := []struct {
		method string
		want   time.Duration
	}{
		// CodeIntel read methods: 90s
		{"codeintel.subgraph", 90 * time.Second},
		{"codeintel.overview", 90 * time.Second},
		{"codeintel.processes", 90 * time.Second},
		{"codeintel.process", 90 * time.Second},
		{"codeintel.impact", 90 * time.Second},
		{"codeintel.symbol", 90 * time.Second},
		{"codeintel.routes", 90 * time.Second},
		{"codeintel.detectChanges", 90 * time.Second},
		{"codeintel.structuralFacts", 90 * time.Second},

		// CodeIntel status / lifecycle methods: 0 (default 30s)
		{"codeintel.status", 0},
		{"codeintel.reindex", 0},
		{"codeintel.reindexStatus", 0},
		{"codeintel.reindexCancel", 0},
		{"codeintel.watch", 0},

		// Quality methods
		{"quality.listProfiles", 45 * time.Second},
		{"quality.run", 0},
		{"quality.runStatus", 0},
		{"quality.cancel", 0},
		{"quality.results", 0},
		{"quality.coverage", 0},

		// Agent exec prompt
		{"agent.execPrompt", 15 * time.Minute},

		// Generic non-codeintel methods
		{"ports.scan", 0},
		{"preflight.check", 0},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := execTimeoutForMethod(tt.method)
			if got != tt.want {
				t.Errorf("execTimeoutForMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestClientExec_CodeIntelSubgraphSurvivesLongerThanRequestTimeout(t *testing.T) {
	agent := &fakeAgent{
		t:            t,
		requireToken: fakeAgentToken,
		results: map[string]any{
			"codeintel.subgraph": map[string]any{"nodes": []any{}},
			"codeintel.status":   map[string]any{"state": "ready"},
		},
		responseDelay: map[string]time.Duration{
			"codeintel.subgraph": 250 * time.Millisecond,
			"codeintel.status":   250 * time.Millisecond,
		},
	}
	host, port := startFakeAgent(t, agent)

	cfg := testConfig(port)
	cfg.RequestTimeout = 100 * time.Millisecond // shorter than 250ms simulated delay
	client := New(cfg, slog.Default(), WithAgentTokens(fakeStaticTokenSource{token: fakeAgentToken}))
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-t1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	// codeintel.subgraph has 90s override, so it survives 250ms delay
	res, err := client.Exec(context.Background(), devServer, "codeintel.subgraph", map[string]any{"center": "foo"})
	if err != nil {
		t.Fatalf("expected codeintel.subgraph to succeed with 90s timeout override, got: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result from codeintel.subgraph")
	}

	// codeintel.status has no override (uses RequestTimeout=100ms), so it times out
	_, err = client.Exec(context.Background(), devServer, "codeintel.status", nil)
	if err == nil {
		t.Fatal("expected codeintel.status to time out under RequestTimeout 100ms, got nil error")
	}
}
