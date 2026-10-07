package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// AgentTarget encapsulates the routing tuple needed to reach a DevServer agent.
type AgentTarget struct {
	TenantID      string
	DevServerID   string
	WorkspaceRoot string
	HostPlatform  string
}

// RawCodeIntelResult is the raw result envelope returned by agent methods (§2.2).
type RawCodeIntelResult struct {
	Sources    []string        `json:"sources,omitempty"`
	HeadCommit string          `json:"headCommit,omitempty"`
	Stale      bool            `json:"stale,omitempty"`
	Truncated  bool            `json:"truncated,omitempty"`
	TotalCount *int64          `json:"totalCount,omitempty"`
	Warnings   []string        `json:"warnings,omitempty"`
	Perf       map[string]any  `json:"perf,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// AgentRPCCaller executes raw JSON-RPC invocations against an agent target.
type AgentRPCCaller interface {
	Call(ctx context.Context, target AgentTarget, method string, params map[string]any) (RawCodeIntelResult, error)
}

// AgentCodeIntelGateway defines typed wrappers for all 16 CodeIntel agent methods.
type AgentCodeIntelGateway interface {
	Status(ctx context.Context, target AgentTarget, p StatusParams) (RawCodeIntelResult, error)
	Overview(ctx context.Context, target AgentTarget, p OverviewParams) (RawCodeIntelResult, error)
	Processes(ctx context.Context, target AgentTarget, p ProcessesParams) (RawCodeIntelResult, error)
	Process(ctx context.Context, target AgentTarget, p ProcessParams) (RawCodeIntelResult, error)
	Subgraph(ctx context.Context, target AgentTarget, p SubgraphParams) (RawCodeIntelResult, error)
	Impact(ctx context.Context, target AgentTarget, p ImpactParams) (RawCodeIntelResult, error)
	Symbol(ctx context.Context, target AgentTarget, p SymbolParams) (RawCodeIntelResult, error)
	Routes(ctx context.Context, target AgentTarget, p RoutesParams) (RawCodeIntelResult, error)
	DetectChanges(ctx context.Context, target AgentTarget, p DetectChangesParams) (RawCodeIntelResult, error)
	StructuralFacts(ctx context.Context, target AgentTarget, p StructuralFactsParams) (RawCodeIntelResult, error)
	Reindex(ctx context.Context, target AgentTarget, p ReindexParams) (RawCodeIntelResult, error)
	ReindexStatus(ctx context.Context, target AgentTarget, p ReindexStatusParams) (RawCodeIntelResult, error)
	ReindexCancel(ctx context.Context, target AgentTarget, p ReindexCancelParams) (RawCodeIntelResult, error)
	Watch(ctx context.Context, target AgentTarget, p WatchParams) (RawCodeIntelResult, error)
	CodegraphSearch(ctx context.Context, target AgentTarget, p CodegraphSearchParams) (RawCodeIntelResult, error)
	Files(ctx context.Context, target AgentTarget, p FilesParams) (RawCodeIntelResult, error)
}

func invalidParamsErr(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", msg, nil)
}

func validateRelativePath(val, fieldName string) error {
	if val == "" {
		return nil
	}
	if strings.HasPrefix(val, "/") || strings.HasPrefix(val, "\\") {
		return invalidParamsErr(fmt.Sprintf("%s must be a relative path", fieldName))
	}
	if strings.HasPrefix(val, "-") {
		return invalidParamsErr(fmt.Sprintf("%s must not start with '-'", fieldName))
	}
	if strings.Contains(val, "..") {
		return invalidParamsErr(fmt.Sprintf("%s must not contain '..'", fieldName))
	}
	return nil
}

// 1. Status
type StatusParams struct {
	BaseRef string
}

func (p StatusParams) Validate() error {
	if p.BaseRef != "" {
		if len(p.BaseRef) > 256 || strings.HasPrefix(p.BaseRef, "-") {
			return invalidParamsErr("baseRef must be <= 256 chars and not start with '-'")
		}
	}
	return nil
}

func (p StatusParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.BaseRef != "" {
		m["baseRef"] = p.BaseRef
	}
	return m
}

// 2. Overview
type OverviewParams struct {
	TopN         int
	MaxEdges     int
	EdgeKinds    []string
	WithTopFiles *bool
}

func (p OverviewParams) Validate() error {
	if p.TopN < 0 || p.TopN > 500 {
		return invalidParamsErr("topN must be between 1 and 500")
	}
	if p.MaxEdges < 0 || p.MaxEdges > 5000 {
		return invalidParamsErr("maxEdges must be between 1 and 5000")
	}
	for _, k := range p.EdgeKinds {
		switch k {
		case "CALLS", "IMPORTS", "ACCESSES", "EXTENDS", "IMPLEMENTS":
		default:
			return invalidParamsErr("invalid edgeKind: " + k)
		}
	}
	return nil
}

func (p OverviewParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.TopN > 0 {
		m["topN"] = p.TopN
	}
	if p.MaxEdges > 0 {
		m["maxEdges"] = p.MaxEdges
	}
	if len(p.EdgeKinds) > 0 {
		m["edgeKinds"] = append([]string(nil), p.EdgeKinds...)
	}
	if p.WithTopFiles != nil {
		m["withTopFiles"] = *p.WithTopFiles
	}
	return m
}

// 3. Processes
type ProcessesParams struct {
	Limit  int
	Offset int
}

func (p ProcessesParams) Validate() error {
	if p.Limit < 0 || p.Limit > 100 {
		return invalidParamsErr("limit must be between 1 and 100")
	}
	if p.Offset < 0 {
		return invalidParamsErr("offset must be >= 0")
	}
	return nil
}

func (p ProcessesParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.Offset > 0 {
		m["offset"] = p.Offset
	}
	return m
}

// 4. Process
type ProcessParams struct {
	ProcessID string
}

func (p ProcessParams) Validate() error {
	if p.ProcessID == "" || len(p.ProcessID) > 128 || strings.HasPrefix(p.ProcessID, "-") {
		return invalidParamsErr("processId is required, <= 128 chars and must not start with '-'")
	}
	return nil
}

func (p ProcessParams) toParams(target AgentTarget) map[string]any {
	return map[string]any{
		"workspaceRoot": target.WorkspaceRoot,
		"processId":     p.ProcessID,
	}
}

// 5. Subgraph
type SubgraphParams struct {
	CenterSymbol  string
	CenterFile    string
	CenterCluster string
	Depth         int
	Kinds         []string
	Limit         int
	Source        string
}

func (p SubgraphParams) Validate() error {
	centers := 0
	if p.CenterSymbol != "" {
		if strings.HasPrefix(p.CenterSymbol, "-") {
			return invalidParamsErr("centerSymbol must not start with '-'")
		}
		centers++
	}
	if p.CenterFile != "" {
		if err := validateRelativePath(p.CenterFile, "centerFile"); err != nil {
			return err
		}
		centers++
	}
	if p.CenterCluster != "" {
		if strings.HasPrefix(p.CenterCluster, "-") {
			return invalidParamsErr("centerCluster must not start with '-'")
		}
		centers++
	}
	if centers != 1 {
		return invalidParamsErr("subgraph center must specify exactly one of symbol, file, or cluster")
	}
	if p.Depth < 0 || p.Depth > 3 {
		return invalidParamsErr("depth must be between 1 and 3")
	}
	if p.Limit < 0 || p.Limit > 1500 {
		return invalidParamsErr("limit must be <= 1500")
	}
	if p.Source != "" {
		switch p.Source {
		case "gitnexus", "codegraph", "auto":
		default:
			return invalidParamsErr("invalid source: " + p.Source)
		}
	}
	return nil
}

func (p SubgraphParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	center := make(map[string]any)
	if p.CenterSymbol != "" {
		center["symbol"] = p.CenterSymbol
	} else if p.CenterFile != "" {
		center["file"] = p.CenterFile
	} else if p.CenterCluster != "" {
		center["cluster"] = p.CenterCluster
	}
	m["center"] = center
	if p.Depth > 0 {
		m["depth"] = p.Depth
	}
	if len(p.Kinds) > 0 {
		m["kinds"] = append([]string(nil), p.Kinds...)
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.Source != "" {
		m["source"] = p.Source
	}
	return m
}

// 6. Impact
type ImpactParams struct {
	UID          string
	Name         string
	File         string
	Kind         string
	Key          string
	Direction    string
	Depth        int
	Limit        int
	IncludeTests bool
}

func (p ImpactParams) Validate() error {
	hasTarget := p.UID != "" || p.Key != "" || p.Name != ""
	if !hasTarget {
		return invalidParamsErr("impact target is required (uid, key, or name)")
	}
	if strings.HasPrefix(p.UID, "-") || strings.HasPrefix(p.Key, "-") || strings.HasPrefix(p.Name, "-") {
		return invalidParamsErr("impact target identifiers must not start with '-'")
	}
	if p.File != "" {
		if err := validateRelativePath(p.File, "file"); err != nil {
			return err
		}
	}
	if p.Direction != "" && p.Direction != "upstream" && p.Direction != "downstream" {
		return invalidParamsErr("direction must be 'upstream' or 'downstream'")
	}
	if p.Depth < 0 || p.Depth > 3 {
		return invalidParamsErr("depth must be between 1 and 3")
	}
	if p.Limit < 0 || p.Limit > 300 {
		return invalidParamsErr("limit must be between 1 and 300")
	}
	return nil
}

func (p ImpactParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	tgt := make(map[string]any)
	if p.UID != "" {
		tgt["uid"] = p.UID
	} else if p.Key != "" {
		tgt["key"] = p.Key
	} else {
		tgt["name"] = p.Name
		if p.File != "" {
			tgt["file"] = p.File
		}
		if p.Kind != "" {
			tgt["kind"] = p.Kind
		}
	}
	m["target"] = tgt
	if p.Direction != "" {
		m["direction"] = p.Direction
	}
	if p.Depth > 0 {
		m["depth"] = p.Depth
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.IncludeTests {
		m["includeTests"] = p.IncludeTests
	}
	return m
}

// 7. Symbol
type SymbolParams struct {
	UID           string
	Name          string
	File          string
	Key           string
	IncludeSource *bool
	RelationLimit int
	IncludeTrail  bool
}

func (p SymbolParams) Validate() error {
	hasID := p.UID != "" || p.Key != "" || (p.Name != "" && p.File != "")
	if !hasID {
		return invalidParamsErr("symbol requires uid, key, or name+file")
	}
	if strings.HasPrefix(p.UID, "-") || strings.HasPrefix(p.Key, "-") || strings.HasPrefix(p.Name, "-") {
		return invalidParamsErr("symbol identifiers must not start with '-'")
	}
	if p.File != "" {
		if err := validateRelativePath(p.File, "file"); err != nil {
			return err
		}
	}
	if p.RelationLimit < 0 || p.RelationLimit > 50 {
		return invalidParamsErr("relationLimit must be between 1 and 50")
	}
	return nil
}

func (p SymbolParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.UID != "" {
		m["uid"] = p.UID
	} else if p.Key != "" {
		m["key"] = p.Key
	} else {
		m["name"] = p.Name
		m["file"] = p.File
	}
	if p.IncludeSource != nil {
		m["includeSource"] = *p.IncludeSource
	}
	if p.RelationLimit > 0 {
		m["relationLimit"] = p.RelationLimit
	}
	if p.IncludeTrail {
		m["includeTrail"] = p.IncludeTrail
	}
	return m
}

// 8. Routes
type RoutesParams struct {
	Limit  int
	Offset int
}

func (p RoutesParams) Validate() error {
	if p.Limit < 0 || p.Limit > 500 {
		return invalidParamsErr("limit must be between 1 and 500")
	}
	if p.Offset < 0 {
		return invalidParamsErr("offset must be >= 0")
	}
	return nil
}

func (p RoutesParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.Offset > 0 {
		m["offset"] = p.Offset
	}
	return m
}

// 9. DetectChanges
type DetectChangesParams struct {
	Base             string
	Head             string
	IncludeUntracked *bool
	WithClusters     bool
	CrossCheck       bool
}

func (p DetectChangesParams) Validate() error {
	if p.Base != "" {
		if len(p.Base) > 256 || strings.HasPrefix(p.Base, "-") {
			return invalidParamsErr("base must be <= 256 chars and not start with '-'")
		}
	}
	if p.Head != "" {
		if strings.HasPrefix(p.Head, "-") {
			return invalidParamsErr("head must not start with '-'")
		}
	}
	return nil
}

func (p DetectChangesParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.Base != "" {
		m["base"] = p.Base
	}
	if p.Head != "" {
		m["head"] = p.Head
	}
	if p.IncludeUntracked != nil {
		m["includeUntracked"] = *p.IncludeUntracked
	}
	if p.WithClusters {
		m["withClusters"] = p.WithClusters
	}
	if p.CrossCheck {
		m["crossCheck"] = p.CrossCheck
	}
	return m
}

// 10. StructuralFacts
type StructuralFactsParams struct {
	FactKind     string
	Pair         string
	PathPrefixes []string
	Limit        int
	Offset       int
}

func (p StructuralFactsParams) Validate() error {
	switch p.FactKind {
	case "layerImports", "cycles", "importInDegree", "fileSizes", "unusedExports":
	default:
		return invalidParamsErr("invalid structural fact kind: " + p.FactKind)
	}
	if len(p.PathPrefixes) > 20 {
		return invalidParamsErr("pathPrefixes must contain <= 20 entries")
	}
	for _, pfx := range p.PathPrefixes {
		if err := validateRelativePath(pfx, "pathPrefix"); err != nil {
			return err
		}
	}
	if p.Limit < 0 || p.Limit > 5000 {
		return invalidParamsErr("limit must be between 1 and 5000")
	}
	if p.Offset < 0 {
		return invalidParamsErr("offset must be >= 0")
	}
	return nil
}

func (p StructuralFactsParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{
		"workspaceRoot": target.WorkspaceRoot,
		"kind":          p.FactKind,
	}
	if p.Pair != "" {
		m["pair"] = p.Pair
	}
	if len(p.PathPrefixes) > 0 {
		m["pathPrefixes"] = append([]string(nil), p.PathPrefixes...)
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.Offset > 0 {
		m["offset"] = p.Offset
	}
	return m
}

// 11. Reindex
type ReindexParams struct {
	Mode          string
	SelectedTools []string
	Trigger       string
	IfStale       bool
	ExpectHead    string
}

func (p ReindexParams) Validate() error {
	if p.Mode != "" && p.Mode != "incremental" && p.Mode != "full" {
		return invalidParamsErr("mode must be 'incremental' or 'full'")
	}
	if p.Trigger != "" {
		switch p.Trigger {
		case "manual", "agent_done", "head_change":
		default:
			return invalidParamsErr("invalid trigger: " + p.Trigger)
		}
	}
	if p.ExpectHead != "" && strings.HasPrefix(p.ExpectHead, "-") {
		return invalidParamsErr("expectHead must not start with '-'")
	}
	for _, t := range p.SelectedTools {
		if t != "gitnexus" && t != "codegraph" {
			return invalidParamsErr("invalid tool in selectedTools: " + t)
		}
	}
	return nil
}

func (p ReindexParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.Mode != "" {
		m["mode"] = p.Mode
	}
	if len(p.SelectedTools) > 0 {
		m["tools"] = append([]string(nil), p.SelectedTools...)
	}
	if p.Trigger != "" {
		m["trigger"] = p.Trigger
	}
	if p.IfStale {
		m["ifStale"] = p.IfStale
	}
	if p.ExpectHead != "" {
		m["expectHead"] = p.ExpectHead
	}
	return m
}

// 12. ReindexStatus
type ReindexStatusParams struct {
	JobID string
}

func (p ReindexStatusParams) Validate() error {
	if p.JobID != "" {
		if len(p.JobID) > 128 || strings.HasPrefix(p.JobID, "-") {
			return invalidParamsErr("jobId must be <= 128 chars and not start with '-'")
		}
	}
	return nil
}

func (p ReindexStatusParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.JobID != "" {
		m["jobId"] = p.JobID
	}
	return m
}

// 13. ReindexCancel
type ReindexCancelParams struct {
	JobID string
}

func (p ReindexCancelParams) Validate() error {
	if p.JobID == "" || len(p.JobID) > 128 || strings.HasPrefix(p.JobID, "-") {
		return invalidParamsErr("jobId is required, <= 128 chars and not start with '-'")
	}
	return nil
}

func (p ReindexCancelParams) toParams(target AgentTarget) map[string]any {
	return map[string]any{
		"workspaceRoot": target.WorkspaceRoot,
		"jobId":         p.JobID,
	}
}

// 14. Watch
type WatchParams struct {
	Enabled bool
}

func (p WatchParams) Validate() error {
	return nil
}

func (p WatchParams) toParams(target AgentTarget) map[string]any {
	return map[string]any{
		"workspaceRoot": target.WorkspaceRoot,
		"enabled":       p.Enabled,
	}
}

// 15. CodegraphSearch
type CodegraphSearchParams struct {
	Query      string
	Limit      int
	SymbolKind string
}

func (p CodegraphSearchParams) Validate() error {
	if p.Query == "" || len(p.Query) > 256 || strings.HasPrefix(p.Query, "-") {
		return invalidParamsErr("query is required, <= 256 chars and not start with '-'")
	}
	if p.Limit < 0 || p.Limit > 50 {
		return invalidParamsErr("limit must be between 1 and 50")
	}
	if p.SymbolKind != "" && strings.HasPrefix(p.SymbolKind, "-") {
		return invalidParamsErr("symbolKind must not start with '-'")
	}
	return nil
}

func (p CodegraphSearchParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{
		"workspaceRoot": target.WorkspaceRoot,
		"search":        p.Query,
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	if p.SymbolKind != "" {
		m["kind"] = p.SymbolKind
	}
	return m
}

// 16. Files
type FilesParams struct {
	Filter string
	Limit  int
}

func (p FilesParams) Validate() error {
	if p.Filter != "" {
		if len(p.Filter) > 512 {
			return invalidParamsErr("filter must be <= 512 chars")
		}
		if err := validateRelativePath(p.Filter, "filter"); err != nil {
			return err
		}
	}
	if p.Limit < 0 || p.Limit > 5000 {
		return invalidParamsErr("limit must be between 1 and 5000")
	}
	return nil
}

func (p FilesParams) toParams(target AgentTarget) map[string]any {
	m := map[string]any{"workspaceRoot": target.WorkspaceRoot}
	if p.Filter != "" {
		m["filter"] = p.Filter
	}
	if p.Limit > 0 {
		m["limit"] = p.Limit
	}
	return m
}

// agentCodeIntelGateway implements AgentCodeIntelGateway using an AgentRPCCaller.
type agentCodeIntelGateway struct {
	caller AgentRPCCaller
}

// NewAgentCodeIntelGateway creates a new AgentCodeIntelGateway wrapping the provided caller.
func NewAgentCodeIntelGateway(caller AgentRPCCaller) AgentCodeIntelGateway {
	return &agentCodeIntelGateway{caller: caller}
}

func (g *agentCodeIntelGateway) Status(ctx context.Context, target AgentTarget, p StatusParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.status", p.toParams(target))
}

func (g *agentCodeIntelGateway) Overview(ctx context.Context, target AgentTarget, p OverviewParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.overview", p.toParams(target))
}

func (g *agentCodeIntelGateway) Processes(ctx context.Context, target AgentTarget, p ProcessesParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.processes", p.toParams(target))
}

func (g *agentCodeIntelGateway) Process(ctx context.Context, target AgentTarget, p ProcessParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.process", p.toParams(target))
}

func (g *agentCodeIntelGateway) Subgraph(ctx context.Context, target AgentTarget, p SubgraphParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.subgraph", p.toParams(target))
}

func (g *agentCodeIntelGateway) Impact(ctx context.Context, target AgentTarget, p ImpactParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.impact", p.toParams(target))
}

func (g *agentCodeIntelGateway) Symbol(ctx context.Context, target AgentTarget, p SymbolParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.symbol", p.toParams(target))
}

func (g *agentCodeIntelGateway) Routes(ctx context.Context, target AgentTarget, p RoutesParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.routes", p.toParams(target))
}

func (g *agentCodeIntelGateway) DetectChanges(ctx context.Context, target AgentTarget, p DetectChangesParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.detectChanges", p.toParams(target))
}

func (g *agentCodeIntelGateway) StructuralFacts(ctx context.Context, target AgentTarget, p StructuralFactsParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.structuralFacts", p.toParams(target))
}

func (g *agentCodeIntelGateway) Reindex(ctx context.Context, target AgentTarget, p ReindexParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.reindex", p.toParams(target))
}

func (g *agentCodeIntelGateway) ReindexStatus(ctx context.Context, target AgentTarget, p ReindexStatusParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.reindexStatus", p.toParams(target))
}

func (g *agentCodeIntelGateway) ReindexCancel(ctx context.Context, target AgentTarget, p ReindexCancelParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.reindexCancel", p.toParams(target))
}

func (g *agentCodeIntelGateway) Watch(ctx context.Context, target AgentTarget, p WatchParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.watch", p.toParams(target))
}

func (g *agentCodeIntelGateway) CodegraphSearch(ctx context.Context, target AgentTarget, p CodegraphSearchParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.codegraphSearch", p.toParams(target))
}

func (g *agentCodeIntelGateway) Files(ctx context.Context, target AgentTarget, p FilesParams) (RawCodeIntelResult, error) {
	if err := p.Validate(); err != nil {
		return RawCodeIntelResult{}, err
	}
	return g.caller.Call(ctx, target, "codeintel.files", p.toParams(target))
}
