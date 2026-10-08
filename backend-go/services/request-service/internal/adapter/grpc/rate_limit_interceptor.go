package grpc

import (
	"context"
	"sync/atomic"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// RateLimitedTotal counts refusals (request_rate_limited_total); the class is not a tenant label on purpose.
var RateLimitedTotal atomic.Int64

// RateLimitInterceptor enforces the per-class token buckets and the DB-backed concurrency caps. It runs after
// RequestAccessInterceptor so refused callers cost no tokens and the target project is already in ctx.
func RateLimitInterceptor(l *usecase.RateLimiter, gate *usecase.ConcurrencyGate) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		e, ok := domain.Catalog[info.FullMethod]
		if !ok || e.RateClass == domain.RateNone {
			return handler(ctx, req)
		}
		tenantID, err := tenant.RequireTenantID(ctx)
		if err != nil {
			return nil, apperrors.ToGRPCStatus(domain.ErrRequestTenantRequired())
		}
		class, subject := effectiveRateClass(ctx, info.FullMethod, e, req)
		if ok, wait := l.Allow(tenantID, class, subject); !ok {
			return nil, rateLimitedStatus(&domain.RateLimitedError{Class: class, Detail: "rate", RetryAfter: wait})
		}
		if err := checkConcurrency(ctx, gate, info.FullMethod, e, class); err != nil {
			if rl, ok := domain.IsRateLimited(err); ok {
				return nil, rateLimitedStatus(rl)
			}
			return nil, apperrors.ToGRPCStatus(err)
		}
		return handler(ctx, req)
	}
}

// effectiveRateClass sends machine-made Requests (webhook, MCP) to the webhook class, one bucket per MCP client.
func effectiveRateClass(ctx context.Context, method string, e domain.Entry, req any) (class, subject string) {
	if method == "/orca.request.v1.RequestService/CreateRequest" {
		if m, ok := req.(*requestv1.CreateRequestRequest); ok {
			switch m.GetSource().GetProvider() {
			case "webhook", "mcp":
				class = domain.RateWebhook
			}
		}
	}
	if class == "" {
		class = e.RateClass
	}
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		subject, _ = tenant.UserID(ctx)
	}
	return class, subject
}

func checkConcurrency(ctx context.Context, gate *usecase.ConcurrencyGate, method string, e domain.Entry, class string) error {
	if gate == nil {
		return nil
	}
	switch {
	case method == "/orca.request.v1.RequestService/CreateRequest":
		return gate.CheckCreate(ctx)
	case e.RateClass == domain.RateAI:
		return gate.CheckRun(ctx, usecase.AccessProject(ctx))
	}
	return nil
}

func rateLimitedStatus(rl *domain.RateLimitedError) error {
	RateLimitedTotal.Add(1)
	st := status.New(codes.ResourceExhausted, "REQUEST_RATE_LIMITED: "+rl.Class+" "+rl.Detail)
	if d, err := st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(rl.RetryAfter)}); err == nil {
		st = d
	}
	return st.Err()
}
