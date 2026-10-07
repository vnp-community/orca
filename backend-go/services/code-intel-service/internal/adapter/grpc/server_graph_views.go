package grpc

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

// CodeIntelServer implements codeintelv1.CodeIntelServiceServer.
type CodeIntelServer struct {
	codeintelv1.UnimplementedCodeIntelServiceServer
	viewReader     usecase.ViewReader
	resolver       usecase.WorktreeTargetResolver
	requestReindex *usecase.RequestReindexUseCase
	getReindexJob  *usecase.GetReindexJobUseCase
	metrics        *domain.CollectorMetrics
	broadcaster    *broadcaster.CodeIntelPushBroadcaster
	pushAuthz      *usecase.PushAuthorization
}

// NewCodeIntelServer creates a new CodeIntelServer.
func NewCodeIntelServer(
	viewReader usecase.ViewReader,
	resolver usecase.WorktreeTargetResolver,
	requestReindex *usecase.RequestReindexUseCase,
	getReindexJob *usecase.GetReindexJobUseCase,
	metrics *domain.CollectorMetrics,
) *CodeIntelServer {
	if metrics == nil {
		metrics = domain.NewCollectorMetrics()
	}
	return &CodeIntelServer{
		viewReader:     viewReader,
		resolver:       resolver,
		requestReindex: requestReindex,
		getReindexJob:  getReindexJob,
		metrics:        metrics,
	}
}

func (s *CodeIntelServer) resolveTenantAndTarget(ctx context.Context, sel *codeintelv1.WorktreeSelector) (string, usecase.AgentTarget, error) {
	tenantID, ok := tenant.TenantID(ctx)
	if !ok || tenantID == "" {
		if sel != nil && sel.ProjectId != "" {
			tenantID = sel.ProjectId
		} else {
			return "", usecase.AgentTarget{}, apperrors.New(apperrors.KindUnauthenticated, "UNAUTHENTICATED", "missing tenant identity in context", nil)
		}
	}

	if sel == nil {
		return "", usecase.AgentTarget{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", "selector is required", nil)
	}

	if s.resolver == nil {
		target := usecase.AgentTarget{
			TenantID:      tenantID,
			WorkspaceRoot: sel.WorktreeRef,
		}
		return tenantID, target, nil
	}

	target, _, err := s.resolver.ResolveTarget(ctx, tenantID, sel)
	if err != nil {
		return "", usecase.AgentTarget{}, err
	}
	return tenantID, target, nil
}

func (s *CodeIntelServer) GetStructure(ctx context.Context, req *codeintelv1.GetStructureRequest) (*codeintelv1.GetStructureResponse, error) {
	start := time.Now()
	method := "GetStructure"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	params := usecase.SubgraphParams{
		CenterFile: req.GetPath(),
		Depth:      int(req.GetDepth()),
		Limit:      int(req.GetLimit()),
	}

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindStructure, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	mg, ok := res.Data.(domain.ModuleGraph)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for structure view", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("structure")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetStructureResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoModuleGraph(mg),
	}, nil
}

func (s *CodeIntelServer) GetClusterOverview(ctx context.Context, req *codeintelv1.GetClusterOverviewRequest) (*codeintelv1.GetClusterOverviewResponse, error) {
	start := time.Now()
	method := "GetClusterOverview"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	params := usecase.OverviewParams{
		TopN: int(req.GetTopN()),
	}

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindClusters, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	arch, ok := res.Data.(domain.ArchitectureGraph)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for cluster overview", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("clusters")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetClusterOverviewResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoArchitectureGraph(arch),
	}, nil
}

func (s *CodeIntelServer) GetSubgraph(ctx context.Context, req *codeintelv1.GetSubgraphRequest) (*codeintelv1.GetSubgraphResponse, error) {
	start := time.Now()
	method := "GetSubgraph"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	var params usecase.SubgraphParams
	switch c := req.GetCenter().(type) {
	case *codeintelv1.GetSubgraphRequest_Symbol:
		params.CenterSymbol = c.Symbol
	case *codeintelv1.GetSubgraphRequest_File:
		params.CenterFile = c.File
	case *codeintelv1.GetSubgraphRequest_Cluster:
		params.CenterCluster = c.Cluster
	}
	params.Depth = int(req.GetDepth())
	params.Kinds = req.GetKinds()
	params.Limit = int(req.GetLimit())

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindSubgraph, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	sg, ok := res.Data.(domain.SymbolGraph)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for subgraph", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("subgraph")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetSubgraphResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoSymbolGraph(sg),
	}, nil
}

func (s *CodeIntelServer) GetImpact(ctx context.Context, req *codeintelv1.GetImpactRequest) (*codeintelv1.GetImpactResponse, error) {
	start := time.Now()
	method := "GetImpact"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	var params usecase.ImpactParams
	switch t := req.GetTarget().(type) {
	case *codeintelv1.GetImpactRequest_Key:
		params.Key = t.Key
	case *codeintelv1.GetImpactRequest_Named:
		if t.Named != nil {
			params.Name = t.Named.Name
			params.File = t.Named.File
			params.Kind = t.Named.Kind
		}
	}
	params.Direction = req.GetDirection()
	params.Depth = int(req.GetDepth())
	params.IncludeTests = req.GetIncludeTests()

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindImpact, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	ig, ok := res.Data.(domain.ImpactGraph)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for impact graph", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("impact")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetImpactResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoImpactGraph(ig),
	}, nil
}

func (s *CodeIntelServer) GetSymbol(ctx context.Context, req *codeintelv1.GetSymbolRequest) (*codeintelv1.GetSymbolResponse, error) {
	start := time.Now()
	method := "GetSymbol"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	var params usecase.SymbolParams
	switch t := req.GetTarget().(type) {
	case *codeintelv1.GetSymbolRequest_Key:
		params.Key = t.Key
	case *codeintelv1.GetSymbolRequest_Named:
		if t.Named != nil {
			params.Name = t.Named.Name
			params.File = t.Named.File
		}
	}
	inc := req.GetIncludeSource()
	params.IncludeSource = &inc

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindSymbol, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	sd, ok := res.Data.(domain.SymbolDetail)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for symbol detail", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("symbol")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetSymbolResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoSymbolDetail(sd),
	}, nil
}

func (s *CodeIntelServer) GetRouteMap(ctx context.Context, req *codeintelv1.GetRouteMapRequest) (*codeintelv1.GetRouteMapResponse, error) {
	start := time.Now()
	method := "GetRouteMap"

	_, target, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	params := usecase.RoutesParams{
		Limit: int(req.GetLimit()),
	}

	res, err := s.viewReader.Get(ctx, target, domain.ViewKindRoutes, params)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	rm, ok := res.Data.(domain.RouteMap)
	if !ok {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_INVALID_RESULT", "unexpected data type for route map", nil))
	}

	if res.Meta.Truncated {
		s.metrics.RecordTruncated("routes")
	}
	s.metrics.RecordCall(method, "success")

	return &codeintelv1.GetRouteMapResponse{
		Meta: ToProtoResultMeta(res.Meta),
		Data: ToProtoRouteMap(rm),
	}, nil
}
