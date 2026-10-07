package wscompat

import (
	"fmt"
	"time"
)

const (
	codeIntelReadTimeout    = 20 * time.Second
	codeIntelStateTimeout   = 8 * time.Second
	codeIntelSummaryTimeout = 24 * time.Second

	codeIntelDefaultMaxArgsBytes = 16 << 10
	codeIntelSymbolMaxResponse   = 320 << 10

	codeIntelMaxSelectors = 50
	codeIntelMaxKinds     = 32
)

type codeIntelChannelSpec struct {
	Name         string
	Stream       bool
	Timeout      time.Duration
	MaxArgsBytes int
	AllowDevice  bool
	Quality      bool
	MaxResponse  int
	RPC          string
}

var codeIntelChannelCatalog = []codeIntelChannelSpec{
	// 3.1 codeIntel.* (26)
	{Name: "codeIntel.status", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetIndexStatus"},
	{Name: "codeIntel.reindex", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/RequestReindex"},
	{Name: "codeIntel.reindexStatus", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetReindexJob"},
	{Name: "codeIntel.structure", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetStructure"},
	{Name: "codeIntel.architecture", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetArchitecture"},
	{Name: "codeIntel.dataFlows", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/ListDataFlows"},
	{Name: "codeIntel.dataFlow", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetDataFlow"},
	{Name: "codeIntel.erd", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetErd"},
	{Name: "codeIntel.storage", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetStorageMap"},
	{Name: "codeIntel.subgraph", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetSubgraph"},
	{Name: "codeIntel.impact", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetImpact"},
	{Name: "codeIntel.symbol", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, MaxResponse: codeIntelSymbolMaxResponse, RPC: "CodeIntelService/GetSymbol"},
	{Name: "codeIntel.routes", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetRouteMap"},
	{Name: "codeIntel.changeOverlay", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetChangeOverlay"},
	{Name: "codeIntel.readingOrder", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetReadingOrder"},
	{Name: "codeIntel.findings", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/ListFindings"},
	{Name: "codeIntel.dismissFinding", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/DismissFinding"},
	{Name: "codeIntel.contractDiff", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetContractDiff"},
	{Name: "codeIntel.reviewState.get", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetReviewState"},
	{Name: "codeIntel.reviewState.save", Timeout: codeIntelStateTimeout, MaxArgsBytes: 256 << 10, RPC: "CodeIntelService/SaveReviewState"},
	{Name: "codeIntel.c4.get", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/GetC4Overrides"},
	{Name: "codeIntel.c4.save", Timeout: codeIntelStateTimeout, MaxArgsBytes: 96 << 10, RPC: "CodeIntelService/SaveC4Overrides"},
	{Name: "codeIntel.bindRepo", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/BindRepo"},
	{Name: "codeIntel.settings.get", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, AllowDevice: true, RPC: "CodeIntelService/GetSettings"},
	{Name: "codeIntel.settings.set", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, RPC: "CodeIntelService/SetSettings"},
	{Name: "codeIntel.subscribe", Stream: true, RPC: "CodeIntelService/StreamCodeIntelEvents"},

	// 3.2 codeIntel.quality.* (20)
	{Name: "codeIntel.quality.start", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/StartQualityRun"},
	{Name: "codeIntel.quality.cancel", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/CancelQualityRun"},
	{Name: "codeIntel.quality.run", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetQualityRun"},
	{Name: "codeIntel.quality.runs", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/ListQualityRuns"},
	{Name: "codeIntel.quality.findings", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/ListQualityFindings"},
	{Name: "codeIntel.quality.waive", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/WaiveFinding"},
	{Name: "codeIntel.quality.gate", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetQualityGate"},
	{Name: "codeIntel.quality.profile.get", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetQualityProfile"},
	{Name: "codeIntel.quality.profile.save", Timeout: codeIntelStateTimeout, MaxArgsBytes: 96 << 10, Quality: true, RPC: "QualityGateService/SaveQualityProfile"},
	{Name: "codeIntel.quality.trend", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetQualityTrend"},
	{Name: "codeIntel.quality.coverage", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetCoverage"},
	{Name: "codeIntel.quality.trace", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetRequirementTrace"},
	{Name: "codeIntel.quality.trace.confirm", Timeout: codeIntelStateTimeout, MaxArgsBytes: 8 << 10, Quality: true, RPC: "QualityGateService/ConfirmRequirementEvidence"},
	{Name: "codeIntel.quality.trace.link", Timeout: codeIntelStateTimeout, MaxArgsBytes: 8 << 10, Quality: true, RPC: "QualityGateService/LinkWorktreeTask"},
	{Name: "codeIntel.quality.summary", Timeout: codeIntelSummaryTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GenerateReviewSummary"},
	{Name: "codeIntel.quality.report", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/ExportReviewReport"},
	{Name: "codeIntel.quality.ci", Timeout: codeIntelReadTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/RefreshCiRun"},
	{Name: "codeIntel.quality.turn.record", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/RecordAgentTurn"},
	{Name: "codeIntel.quality.turns", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/ListAgentTurns"},
	{Name: "codeIntel.quality.turn", Timeout: codeIntelStateTimeout, MaxArgsBytes: codeIntelDefaultMaxArgsBytes, Quality: true, RPC: "QualityGateService/GetAgentTurn"},
}

func mustCatalogSpec(name string) codeIntelChannelSpec {
	for _, spec := range codeIntelChannelCatalog {
		if spec.Name == name {
			return spec
		}
	}
	panic(fmt.Sprintf("unknown codeIntel channel: %s", name))
}
