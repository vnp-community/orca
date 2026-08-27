// Package grpc implements the generated usagev1.UsageServiceServer interface
// by translating wire messages to/from usecase calls — no business logic
// here, per specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
	"github.com/stablyai/orca-go/services/usage-service/internal/usecase"

	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
)

// Server implements usagev1.UnimplementedUsageServiceServer.
type Server struct {
	usagev1.UnimplementedUsageServiceServer

	recordUsageSession *usecase.RecordUsageSession
	getDailyUsage      *usecase.GetDailyUsage
	listSessions       *usecase.ListSessions
}

func New(record *usecase.RecordUsageSession, daily *usecase.GetDailyUsage, list *usecase.ListSessions) *Server {
	return &Server{recordUsageSession: record, getDailyUsage: daily, listSessions: list}
}

func (s *Server) RecordUsageSession(ctx context.Context, req *usagev1.RecordUsageSessionRequest) (*usagev1.RecordUsageSessionResponse, error) {
	in := req.GetSession()
	session, err := s.recordUsageSession.Execute(ctx, usecase.RecordUsageSessionInput{
		ID:               in.GetId(),
		Provider:         toDomainProvider(in.GetProvider()),
		WorktreeID:       in.GetWorktreeId(),
		InputTokens:      in.GetInputTokens(),
		OutputTokens:     in.GetOutputTokens(),
		CacheReadTokens:  in.GetCacheReadTokens(),
		CacheWriteTokens: in.GetCacheWriteTokens(),
		CostUSD:          in.GetCostUsd(),
		StartedAt:        toUnix(in.GetStartedAt()),
		EndedAt:          toUnix(in.GetEndedAt()),
		RequestID:        in.GetRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &usagev1.RecordUsageSessionResponse{Session: toProtoSession(session)}, nil
}

func (s *Server) GetDailyUsage(ctx context.Context, req *usagev1.GetDailyUsageRequest) (*usagev1.GetDailyUsageResponse, error) {
	rollup, err := s.getDailyUsage.Execute(ctx, usecase.GetDailyUsageInput{
		UserID:   req.GetUserId(),
		Provider: toDomainProvider(req.GetProvider()),
		Day:      toUnix(req.GetDay()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &usagev1.GetDailyUsageResponse{Rollup: toProtoRollup(rollup)}, nil
}

func (s *Server) ListSessions(ctx context.Context, req *usagev1.ListSessionsRequest) (*usagev1.ListSessionsResponse, error) {
	out, err := s.listSessions.Execute(ctx, usecase.ListSessionsInput{
		UserID:    req.GetUserId(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	sessions := make([]*usagev1.UsageSession, 0, len(out.Sessions))
	for _, s := range out.Sessions {
		sessions = append(sessions, toProtoSession(s))
	}
	return &usagev1.ListSessionsResponse{Sessions: sessions, NextPageToken: out.NextPageToken}, nil
}

func toDomainProvider(p usagev1.Provider) domain.Provider {
	switch p {
	case usagev1.Provider_PROVIDER_CLAUDE:
		return domain.ProviderClaude
	case usagev1.Provider_PROVIDER_CODEX:
		return domain.ProviderCodex
	case usagev1.Provider_PROVIDER_OPENCODE:
		return domain.ProviderOpenCode
	default:
		return ""
	}
}

func toProtoProvider(p domain.Provider) usagev1.Provider {
	switch p {
	case domain.ProviderClaude:
		return usagev1.Provider_PROVIDER_CLAUDE
	case domain.ProviderCodex:
		return usagev1.Provider_PROVIDER_CODEX
	case domain.ProviderOpenCode:
		return usagev1.Provider_PROVIDER_OPENCODE
	default:
		return usagev1.Provider_PROVIDER_UNSPECIFIED
	}
}

func toUnix(ts *timestamppb.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.AsTime().Unix()
}

func toProtoSession(s domain.UsageSession) *usagev1.UsageSession {
	out := &usagev1.UsageSession{
		Id:               s.ID,
		TenantId:         s.TenantID,
		UserId:           s.UserID,
		Provider:         toProtoProvider(s.Provider),
		WorktreeId:       s.WorktreeID,
		InputTokens:      s.InputTokens,
		OutputTokens:     s.OutputTokens,
		CacheReadTokens:  s.CacheReadTokens,
		CacheWriteTokens: s.CacheWriteTokens,
		CostUsd:          s.CostUSD,
		RequestId:        s.RequestID,
	}
	if !s.StartedAt.IsZero() {
		out.StartedAt = timestamppb.New(s.StartedAt)
	}
	if !s.EndedAt.IsZero() {
		out.EndedAt = timestamppb.New(s.EndedAt)
	}
	return out
}

func toProtoRollup(r domain.DailyUsageRollup) *usagev1.DailyUsageRollup {
	return &usagev1.DailyUsageRollup{
		TenantId:          r.TenantID,
		UserId:            r.UserID,
		Provider:          toProtoProvider(r.Provider),
		Day:               timestamppb.New(r.Day),
		TotalInputTokens:  r.TotalInputTokens,
		TotalOutputTokens: r.TotalOutputTokens,
		TotalCostUsd:      r.TotalCostUSD,
		SessionCount:      r.SessionCount,
	}
}
