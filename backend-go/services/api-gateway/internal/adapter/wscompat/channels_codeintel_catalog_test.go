package wscompat

import (
	"testing"
	"time"
)

// Hardcoded contract list from UI-API §3 (CONTRACT-codeintel-ui-api.md lines 117-175).
var expectedContractChannels = []struct {
	name         string
	isStream     bool
	timeout      time.Duration
	maxArgsBytes int
	allowDevice  bool
	quality      bool
}{
	// 3.1 codeIntel.* (26 channels)
	{name: "codeIntel.status", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.reindex", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.reindexStatus", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.structure", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.architecture", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.dataFlows", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.dataFlow", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.erd", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.storage", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.subgraph", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.impact", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.symbol", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.routes", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.changeOverlay", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.readingOrder", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.findings", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.dismissFinding", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.contractDiff", timeout: 20 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.reviewState.get", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.reviewState.save", timeout: 8 * time.Second, maxArgsBytes: 256 << 10},
	{name: "codeIntel.c4.get", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.c4.save", timeout: 8 * time.Second, maxArgsBytes: 96 << 10},
	{name: "codeIntel.bindRepo", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.settings.get", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, allowDevice: true},
	{name: "codeIntel.settings.set", timeout: 8 * time.Second, maxArgsBytes: 16 << 10},
	{name: "codeIntel.subscribe", isStream: true},

	// 3.2 codeIntel.quality.* (20 channels)
	{name: "codeIntel.quality.start", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.cancel", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.run", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.runs", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.findings", timeout: 20 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.waive", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.gate", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.profile.get", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.profile.save", timeout: 8 * time.Second, maxArgsBytes: 96 << 10, quality: true},
	{name: "codeIntel.quality.trend", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.coverage", timeout: 20 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.trace", timeout: 20 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.trace.confirm", timeout: 8 * time.Second, maxArgsBytes: 8 << 10, quality: true},
	{name: "codeIntel.quality.trace.link", timeout: 8 * time.Second, maxArgsBytes: 8 << 10, quality: true},
	{name: "codeIntel.quality.summary", timeout: 24 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.report", timeout: 20 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.ci", timeout: 20 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.turn.record", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.turns", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
	{name: "codeIntel.quality.turn", timeout: 8 * time.Second, maxArgsBytes: 16 << 10, quality: true},
}

func TestCodeIntelCatalog_MatchesContract(t *testing.T) {
	if len(codeIntelChannelCatalog) != 46 {
		t.Fatalf("expected 46 channels in catalog, got %d", len(codeIntelChannelCatalog))
	}
	if len(expectedContractChannels) != 46 {
		t.Fatalf("expected 46 channels in contract expectation, got %d", len(expectedContractChannels))
	}

	seen := make(map[string]bool)
	for _, spec := range codeIntelChannelCatalog {
		if seen[spec.Name] {
			t.Errorf("duplicate channel in catalog: %s", spec.Name)
		}
		seen[spec.Name] = true
	}

	for _, exp := range expectedContractChannels {
		spec := mustCatalogSpec(exp.name)
		if spec.Stream != exp.isStream {
			t.Errorf("%s: stream mismatch: got %v, want %v", exp.name, spec.Stream, exp.isStream)
		}
		if !exp.isStream {
			if spec.Timeout != exp.timeout {
				t.Errorf("%s: timeout mismatch: got %v, want %v", exp.name, spec.Timeout, exp.timeout)
			}
			if spec.MaxArgsBytes != exp.maxArgsBytes {
				t.Errorf("%s: maxArgsBytes mismatch: got %d, want %d", exp.name, spec.MaxArgsBytes, exp.maxArgsBytes)
			}
		}
		if spec.AllowDevice != exp.allowDevice {
			t.Errorf("%s: allowDevice mismatch: got %v, want %v", exp.name, spec.AllowDevice, exp.allowDevice)
		}
		if spec.Quality != exp.quality {
			t.Errorf("%s: quality mismatch: got %v, want %v", exp.name, spec.Quality, exp.quality)
		}
	}
}
