package main

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/internalcaller"
	authgrpc "github.com/stablyai/orca-go/services/auth-service/internal/adapter/grpc"
	svcconfig "github.com/stablyai/orca-go/services/auth-service/internal/config"
	"google.golang.org/grpc"
)

// internalCallerGuards puts the internal-only RPCs behind their shared secrets, on top of the NetworkPolicy allow-list
// (no mesh peer identity exists yet). A secret left empty never switches its guard off: internalcaller.Guard refuses
// every call to the methods, so a forgotten variable shows up as a refused RPC instead of an open one.
func internalCallerGuards(cfg svcconfig.OAuthConfig, log *slog.Logger) []grpc.UnaryServerInterceptor {
	if cfg.Enabled && cfg.InternalCallerToken == "" {
		log.Error("OAUTH_INTERNAL_CALLER_TOKEN is empty: the mcp-service-only OAuth RPCs refuse every call")
	}
	if cfg.McpPrincipalCallerToken == "" {
		level := slog.LevelWarn
		if cfg.Enabled {
			level = slog.LevelError
		}
		log.Log(context.Background(), level, "AUTH_MCP_PRINCIPAL_CALLER_TOKEN is empty: ResolveMcpPrincipal refuses every call")
	}
	return []grpc.UnaryServerInterceptor{
		internalcaller.Guard(cfg.InternalCallerToken, append(append([]string{}, authgrpc.OAuthInternalMethods...), authgrpc.McpSuspensionMethods...)...),
		internalcaller.Guard(cfg.McpPrincipalCallerToken, authgrpc.McpPrincipalMethods...),
	}
}

// directoryCallerGuard puts ListActiveTenantAdmins behind its own secret; an empty secret refuses every call.
func directoryCallerGuard(token string, log *slog.Logger) grpc.UnaryServerInterceptor {
	if token == "" {
		log.Warn("AUTH_DIRECTORY_CALLER_TOKEN is empty: ListActiveTenantAdmins refuses every call, so request-service cannot expand role:admin approvers")
	}
	return internalcaller.Guard(token, authgrpc.DirectoryInternalMethods...)
}
