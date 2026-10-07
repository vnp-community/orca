package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

// ViewResult holds the orchestrated query response and metadata envelope.
type ViewResult struct {
	Meta domain.ResultMeta
	Data any
}

// ViewReader defines the interface for retrieving code intelligence views.
type ViewReader interface {
	Get(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (ViewResult, error)
}

// CollectView coordinates view data collection from DevServer agents,
// combining data results with parallel status probes, applying domain limits,
// and enforcing fail-closed source redaction.
type CollectView struct {
	gateway  AgentCodeIntelGateway
	redactor SymbolSourceRedactor
	sizeFn   domain.SizeFunc
}

// NewCollectView creates a new CollectView instance.
func NewCollectView(gateway AgentCodeIntelGateway, redactor SymbolSourceRedactor, sizeFn domain.SizeFunc) *CollectView {
	if redactor == nil {
		redactor = NewFailClosedSymbolSourceRedactor()
	}
	return &CollectView{
		gateway:  gateway,
		redactor: redactor,
		sizeFn:   sizeFn,
	}
}

// Get executes view data fetching, parallel status probing, decoding, limiting, and redaction.
func (c *CollectView) Get(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (ViewResult, error) {
	if c.gateway == nil {
		return ViewResult{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_GATEWAY_NIL", "agent gateway is nil", nil)
	}

	// 1. STATUS view - single call to Status
	if view == domain.ViewKindStatus {
		p := StatusParams{}
		if sp, ok := params.(StatusParams); ok {
			p = sp
		} else if spp, ok := params.(*StatusParams); ok && spp != nil {
			p = *spp
		}
		raw, err := c.gateway.Status(ctx, target, p)
		if err != nil {
			return ViewResult{}, err
		}
		var statusData domain.AgentStatusData
		if len(raw.Data) > 0 {
			if err := json.Unmarshal(raw.Data, &statusData); err != nil {
				return ViewResult{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode status data: %v", err), err)
			}
		}
		sources := extractSourcesFromStatus(&statusData, raw.Sources)
		meta := c.buildResultMeta(target, view, raw, false, 0, sources, nil)
		return ViewResult{Meta: meta, Data: statusData}, nil
	}

	// 2. Views without parallel status call (SYMBOL, FLOWS, FLOW)
	if view == domain.ViewKindSymbol || view == domain.ViewKindFlows || view == domain.ViewKindFlow {
		return c.getDirectView(ctx, target, view, params)
	}

	// 3. Views with parallel status call (CLUSTERS, STRUCTURE, SUBGRAPH, IMPACT, ROUTES, CHANGE_OVERLAY)
	var (
		rawMain     RawCodeIntelResult
		errMain     error
		statusData  *domain.AgentStatusData
		statusWarns []string
		wg          sync.WaitGroup
	)

	wg.Add(2)

	// Branch 1: Main data call
	go func() {
		defer wg.Done()
		rawMain, errMain = c.callMainGateway(ctx, target, view, params)
	}()

	// Branch 2: Parallel status call (failure adds status_unavailable warning, doesn't fail view)
	go func() {
		defer wg.Done()
		rawStatus, errStatus := c.gateway.Status(ctx, target, StatusParams{})
		if errStatus != nil {
			statusWarns = append(statusWarns, "status_unavailable")
			return
		}
		var sd domain.AgentStatusData
		if len(rawStatus.Data) > 0 {
			if err := json.Unmarshal(rawStatus.Data, &sd); err == nil {
				statusData = &sd
			}
		}
	}()

	wg.Wait()

	if errMain != nil {
		return ViewResult{}, errMain
	}

	sources := extractSourcesFromStatus(statusData, rawMain.Sources)

	// Process and limit based on view kind
	switch view {
	case domain.ViewKindClusters:
		arch, err := decodeArchitectureGraph(rawMain.Data, target)
		if err != nil {
			return ViewResult{}, err
		}
		limited, truncated, totalBefore, err := domain.LimitArchitecture(arch, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, rawMain, truncated, totalBefore, sources, statusWarns)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindStructure:
		sg, err := decodeSymbolGraph(rawMain.Data, target)
		if err != nil {
			return ViewResult{}, err
		}
		mg := convertSymbolGraphToModuleGraph(sg)
		limited, truncated, totalBefore, err := domain.LimitModuleGraph(mg, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, rawMain, truncated, totalBefore, sources, statusWarns)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindSubgraph:
		sg, err := decodeSymbolGraph(rawMain.Data, target)
		if err != nil {
			return ViewResult{}, err
		}
		limited, truncated, totalBefore, err := domain.LimitSymbolGraph(sg, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, rawMain, truncated, totalBefore, sources, statusWarns)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindImpact:
		ig, err := decodeImpactGraph(rawMain.Data, target)
		if err != nil {
			return ViewResult{}, err
		}
		limited, truncated, totalBefore, err := domain.LimitImpactGraph(ig, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, rawMain, truncated, totalBefore, sources, statusWarns)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindRoutes:
		rm, err := decodeRouteMap(rawMain.Data, target)
		if err != nil {
			return ViewResult{}, err
		}
		limited, truncated, totalBefore, err := domain.LimitRouteMap(rm, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, rawMain, truncated, totalBefore, sources, statusWarns)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindChangeOverlay:
		// Return RawCodeIntelResult directly per Table 2.E (SOL-036 converts)
		meta := c.buildResultMeta(target, view, rawMain, false, 0, sources, statusWarns)
		return ViewResult{Meta: meta, Data: rawMain}, nil

	default:
		return ViewResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_UNKNOWN_VIEW", fmt.Sprintf("unsupported view kind: %v", view), nil)
	}
}

func (c *CollectView) getDirectView(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (ViewResult, error) {
	raw, err := c.callMainGateway(ctx, target, view, params)
	if err != nil {
		return ViewResult{}, err
	}

	var sources []domain.SourceInfo
	for _, s := range raw.Sources {
		sources = append(sources, domain.SourceInfo{
			Tool:     s,
			Commit:   raw.HeadCommit,
			LineBase: 1,
		})
	}

	switch view {
	case domain.ViewKindSymbol:
		detail, err := decodeSymbolDetail(raw.Data, target)
		if err != nil {
			return ViewResult{}, err
		}

		// Apply source redaction
		redactor := c.redactor
		if redactor == nil {
			redactor = NewFailClosedSymbolSourceRedactor()
		}

		redactedSrc, omittedReason, redErr := redactor.RedactSource(ctx, target.WorkspaceRoot, detail.Symbol.FilePath, &detail.Source)
		if redErr != nil {
			return ViewResult{}, redErr
		}

		if omittedReason != "" {
			detail.Source = domain.SymbolSource{}
			detail.SourceOmitted = omittedReason
		} else if redactedSrc != nil {
			detail.Source = *redactedSrc
			detail.SourceOmitted = ""
		}

		limited, truncated, totalBefore, err := domain.LimitSymbolDetail(detail, c.sizeFn)
		if err != nil {
			return ViewResult{}, err
		}
		meta := c.buildResultMeta(target, view, raw, truncated, totalBefore, sources, nil)
		return ViewResult{Meta: meta, Data: limited}, nil

	case domain.ViewKindFlows, domain.ViewKindFlow:
		meta := c.buildResultMeta(target, view, raw, false, 0, sources, nil)
		return ViewResult{Meta: meta, Data: raw}, nil

	default:
		return ViewResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_UNKNOWN_VIEW", fmt.Sprintf("unsupported direct view: %v", view), nil)
	}
}

func (c *CollectView) callMainGateway(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (RawCodeIntelResult, error) {
	switch view {
	case domain.ViewKindClusters:
		p := OverviewParams{}
		if op, ok := params.(OverviewParams); ok {
			p = op
		} else if opp, ok := params.(*OverviewParams); ok && opp != nil {
			p = *opp
		}
		return c.gateway.Overview(ctx, target, p)

	case domain.ViewKindStructure, domain.ViewKindSubgraph:
		p := SubgraphParams{}
		if sp, ok := params.(SubgraphParams); ok {
			p = sp
		} else if spp, ok := params.(*SubgraphParams); ok && spp != nil {
			p = *spp
		}
		return c.gateway.Subgraph(ctx, target, p)

	case domain.ViewKindImpact:
		p := ImpactParams{}
		if ip, ok := params.(ImpactParams); ok {
			p = ip
		} else if ipp, ok := params.(*ImpactParams); ok && ipp != nil {
			p = *ipp
		}
		return c.gateway.Impact(ctx, target, p)

	case domain.ViewKindSymbol:
		p := SymbolParams{}
		if sp, ok := params.(SymbolParams); ok {
			p = sp
		} else if spp, ok := params.(*SymbolParams); ok && spp != nil {
			p = *spp
		}
		return c.gateway.Symbol(ctx, target, p)

	case domain.ViewKindRoutes:
		p := RoutesParams{}
		if rp, ok := params.(RoutesParams); ok {
			p = rp
		} else if rpp, ok := params.(*RoutesParams); ok && rpp != nil {
			p = *rpp
		}
		return c.gateway.Routes(ctx, target, p)

	case domain.ViewKindChangeOverlay:
		p := DetectChangesParams{}
		if dp, ok := params.(DetectChangesParams); ok {
			p = dp
		} else if dpp, ok := params.(*DetectChangesParams); ok && dpp != nil {
			p = *dpp
		}
		return c.gateway.DetectChanges(ctx, target, p)

	case domain.ViewKindFlows:
		p := ProcessesParams{}
		if pp, ok := params.(ProcessesParams); ok {
			p = pp
		} else if ppp, ok := params.(*ProcessesParams); ok && ppp != nil {
			p = *ppp
		}
		return c.gateway.Processes(ctx, target, p)

	case domain.ViewKindFlow:
		p := ProcessParams{}
		if pp, ok := params.(ProcessParams); ok {
			p = pp
		} else if ppp, ok := params.(*ProcessParams); ok && ppp != nil {
			p = *ppp
		}
		return c.gateway.Process(ctx, target, p)

	default:
		return RawCodeIntelResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_UNKNOWN_VIEW", fmt.Sprintf("no gateway method for view kind: %v", view), nil)
	}
}

func (c *CollectView) buildResultMeta(
	target AgentTarget,
	view domain.ViewKind,
	raw RawCodeIntelResult,
	limitTruncated bool,
	totalBefore int64,
	sources []domain.SourceInfo,
	extraWarnings []string,
) domain.ResultMeta {
	isStale := raw.Stale
	if !isStale && raw.HeadCommit != "" {
		for _, s := range sources {
			if s.Commit != "" && s.Commit != raw.HeadCommit {
				isStale = true
				break
			}
		}
	}

	totalCount := totalBefore
	if raw.TotalCount != nil {
		totalCount = *raw.TotalCount
	}

	warnings := make([]string, 0, len(raw.Warnings)+len(extraWarnings))
	warnings = append(warnings, raw.Warnings...)
	warnings = append(warnings, extraWarnings...)

	return domain.ResultMeta{
		Repo:        target.WorkspaceRoot,
		WorktreeRef: target.WorkspaceRoot,
		DevServerID: target.DevServerID,
		Commit:      raw.HeadCommit,
		View:        view,
		GeneratedAt: time.Now().UTC(),
		Sources:     sources,
		Stale:       isStale,
		Truncated:   raw.Truncated || limitTruncated,
		TotalCount:  totalCount,
		TotalBefore: totalBefore,
		Warnings:    warnings,
	}
}

func extractSourcesFromStatus(status *domain.AgentStatusData, fallbackSources []string) []domain.SourceInfo {
	if status == nil {
		var sources []domain.SourceInfo
		for _, s := range fallbackSources {
			sources = append(sources, domain.SourceInfo{
				Tool:     s,
				LineBase: 1,
			})
		}
		return sources
	}

	var sources []domain.SourceInfo
	if gn := status.Indexes.GitNexus; gn != nil {
		var indexedAt time.Time
		if gn.IndexedAt != nil {
			indexedAt = *gn.IndexedAt
		}
		version := ""
		if t, ok := status.Tools["gitnexus"]; ok {
			version = t.Version
		}
		sources = append(sources, domain.SourceInfo{
			Tool:      "gitnexus",
			Version:   version,
			IndexedAt: indexedAt,
			Commit:    gn.IndexedCommit,
			LineBase:  0,
		})
	}

	if cg := status.Indexes.CodeGraph; cg != nil {
		var indexedAt time.Time
		if cg.IndexedAt != nil {
			indexedAt = *cg.IndexedAt
		}
		version := ""
		if t, ok := status.Tools["codegraph"]; ok {
			version = t.Version
		}
		sources = append(sources, domain.SourceInfo{
			Tool:      "codegraph",
			Version:   version,
			IndexedAt: indexedAt,
			Commit:    cg.HeadCommit,
			LineBase:  1,
		})
	}

	if len(sources) == 0 {
		for _, s := range fallbackSources {
			sources = append(sources, domain.SourceInfo{
				Tool:     s,
				LineBase: 1,
			})
		}
	}

	return sources
}
