package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/agentconfig"
	mcpauthclient "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/authclient"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/brokerclient"
	mcpgrpc "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/mcpprober"
	mcppostgres "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/tenantclient"
	svcconfig "github.com/stablyai/orca-go/services/mcp-service/internal/config"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// registryInternalMethods are callable only by trusted internal services
// (infra-fleet at agent spawn, request-service for context sources), guarded
// like the gateway-only governance RPCs.
var registryInternalMethods = []string{
	"/orca.mcp.v1.McpRegistryService/ResolveAgentMcpConfig",
	"/orca.mcp.v1.McpRegistryService/CallExternalTool",
	"/orca.mcp.v1.McpRegistryService/ReadExternalResource",
}

type externalRegistry struct {
	server *mcpgrpc.RegistryServer
	reg    *usecase.ExternalServerRegistry
	cfg    svcconfig.ExternalConfig
	close  []func() error
}

// buildExternalRegistry wires the registry. Missing downstream addresses are
// not fatal: the affected operations answer MCP_UNAVAILABLE instead.
func buildExternalRegistry(repo *mcppostgres.Repository, defaults usecase.Defaults, authz svcconfig.AuthorizationConfig, logger *slog.Logger) (externalRegistry, error) {
	cfg, err := svcconfig.LoadExternal()
	if err != nil {
		return externalRegistry{}, err
	}
	w := externalRegistry{cfg: cfg}
	policy := domain.ExternalURLPolicy{HTTPAllowlist: cfg.HTTPAllowlist, AllowedPorts: cfg.AllowedPorts}
	prober := mcpprober.New(mcpprober.Config{Policy: policy})

	var broker usecase.SecretBroker
	if cfg.BrokerAddr != "" {
		conn, err := brokerclient.Dial(cfg.BrokerAddr)
		if err != nil {
			return w, fmt.Errorf("dialing credential-broker-service: %w", err)
		}
		w.close = append(w.close, conn.Close)
		broker = brokerclient.New(credentialbrokerv1.NewCredentialBrokerServiceClient(conn), 0)
	} else {
		logger.Warn("CREDENTIAL_BROKER_ADDR is empty: external server secrets are unavailable")
	}

	clock := usecase.SystemClock{}
	srvPolicy := domain.ServerPolicy{URL: policy, StdioEnabled: cfg.StdioEnabled}
	w.reg = usecase.NewExternalServerRegistry(repo, broker, prober, usecase.ExternalServerOptions{Policy: srvPolicy, StdioFourEyes: cfg.StdioFourEyes}, clock)

	deps := usecase.ResolveAgentMcpConfigDeps{
		Repo: repo, Broker: broker, Settings: repo, Defaults: defaults, Render: agentconfig.Renderer{}, Egress: prober, Outbox: repo,
	}
	if cfg.TenantServiceAddr != "" {
		conn, err := tenantclient.Dial(cfg.TenantServiceAddr)
		if err != nil {
			return w, fmt.Errorf("dialing tenant-service: %w", err)
		}
		w.close = append(w.close, conn.Close)
		deps.Profile = tenantclient.New(tenantv1.NewTenantServiceClient(conn), 0)
	} else {
		deps.Profile = noProfile{}
	}
	if authz.Enabled {
		conn, err := mcpauthclient.Dial(authz.AuthServiceAddr, authz.InternalToken)
		if err != nil {
			return w, fmt.Errorf("dialing auth-service: %w", err)
		}
		w.close = append(w.close, conn.Close)
		deps.Tokens = mcpauthclient.NewAgentTokenIssuer(mcpauthclient.New(authv1.NewAuthServiceClient(conn), authz.AuthCallDeadline))
	}
	resolve := usecase.NewResolveAgentMcpConfig(deps, usecase.AgentConfigOptions{
		Enabled: cfg.AgentConfigEnabled, MaxDepth: cfg.MaxAgentDepth, TokenTTL: cfg.AgentTokenTTL, TokenScopes: cfg.AgentTokenScopes,
		OrcaMcpURL: cfg.OrcaMcpURL, StdioEnabled: cfg.StdioEnabled, StdioAllowUnsandboxed: cfg.StdioAllowUnsandboxed, URLPolicy: policy,
	}, clock)
	w.server = mcpgrpc.NewRegistryServer(w.reg, resolve, usecase.NewExternalServerClient(repo, broker, prober, repo, clock))
	return w, nil
}

// noProfile is used when tenant-service is not configured: no profile means no
// external server is ever granted to an agent (fail closed).
type noProfile struct{}

func (noProfile) ServerNames(context.Context, string) ([]string, error) { return nil, nil }
func (noProfile) TeamIDs(context.Context, string) ([]string, error)     { return nil, nil }

func (w externalRegistry) closeAll() {
	for _, c := range w.close {
		_ = c()
	}
}

// startHealthWorker runs CheckHealth every MCP_EXTERNAL_HEALTH_INTERVAL/5 (the
// interval is enforced per server by ClaimHealthChecks, so every replica may run it).
func (w externalRegistry) startHealthWorker(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger) {
	if w.cfg.HealthInterval <= 0 {
		return
	}
	tick := w.cfg.HealthInterval / 5
	if tick < 5*time.Second {
		tick = 5 * time.Second
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := w.reg.CheckHealth(ctx, w.cfg.HealthInterval, 20); err != nil {
					logger.WarnContext(ctx, "external server health check failed", slog.Any("error", err))
				} else if n > 0 {
					logger.InfoContext(ctx, "external servers checked", slog.Int("count", n))
				}
			}
		}
	}()
}
