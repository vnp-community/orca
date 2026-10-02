package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcppolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// dialMCPService dials mcp-service and, when MCP_INTERNAL_CALLER_TOKEN is set,
// presents it on every call: mcp-service restricts its gateway-only
// governance RPCs (authorize, complete, decide approval, kill state) to
// callers that know it. Same dev-only transport caveat as gatewaygrpc.Dial.
func dialMCPService(addr string) (*grpc.ClientConn, error) {
	// otelgrpc propagates the MCP request's trace to mcp-service (policy decision and audit share its trace_id).
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
	if tok := os.Getenv("MCP_INTERNAL_CALLER_TOKEN"); tok != "" {
		opts = append(opts, grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok)))
	}
	return grpc.NewClient(addr, opts...)
}

// durationFromEnv parses an optional positive duration; zero means "default".
func durationFromEnv(key string) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive duration", key, v)
	}
	return d, nil
}

// buildMCPGovernance returns the real policy gate and kill guard backed by
// mcp-service. A nil client yields (nil, nil): callers then keep the
// fail-closed default gate and no kill guard. Env: MCP_APPROVAL_MAX_WAIT
// (default 50s), MCP_GOVERNANCE_CALL_TIMEOUT (default 5s).
func buildMCPGovernance(client mcpv1.McpServiceClient, logger *slog.Logger) (mcpserver.PolicyGate, *mcppolicy.KillGuard, error) {
	if client == nil {
		return nil, nil, nil
	}
	wait, err := durationFromEnv("MCP_APPROVAL_MAX_WAIT")
	if err != nil {
		return nil, nil, err
	}
	timeout, err := durationFromEnv("MCP_GOVERNANCE_CALL_TIMEOUT")
	if err != nil {
		return nil, nil, err
	}
	gate := mcppolicy.NewGate(client, mcppolicy.Config{CallTimeout: timeout, ApprovalMaxWait: wait}, logger)
	return gate, mcppolicy.NewKillGuard(client, timeout), nil
}

// withKillGuard refuses kill-switched principals before any handler runs.
func withKillGuard(inner mcpserver.TokenVerifier, guard *mcppolicy.KillGuard) mcpserver.TokenVerifier {
	if guard == nil {
		return inner
	}
	return mcppolicy.KillGuardVerifier{Inner: inner, Guard: guard}
}
