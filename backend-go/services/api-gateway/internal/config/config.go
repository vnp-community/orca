// Package config loads api-gateway's runtime configuration — env/flag
// parsing only, no business logic, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Unlike a data-owning service, api-gateway's config is dominated by
// downstream service addresses (it dials all 16 other services, §7) rather
// than a DATABASE_DSN — this service owns no database (§5).
package config

import (
	"fmt"
	"os"
	"strconv"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config is api-gateway's full runtime configuration.
type Config struct {
	commonconfig.Base

	// PublicPort is where the REST + WS edge listens — the one external
	// listener in the whole system (api-gateway.md §1/§9). Base.GRPCPort is
	// unused by this service (no gRPC server of its own, see
	// cmd/server/main.go); Base.HTTPPort keeps serving /healthz/readyz on
	// the same convention every other service uses.
	PublicPort int

	// UsageServiceAddr and NotificationServiceAddr are the two downstream
	// services this scaffold really dials (see README "what's really
	// wired"): usage-service's REST reverse-proxy reference path and
	// notification-service's WS<->gRPC-stream bridge.
	UsageServiceAddr        string
	NotificationServiceAddr string

	// InfraFleetHTTPAddr is infra-fleet-service's own HTTP listener
	// (common/config's HTTPPort — normally /healthz/readyz/metrics only,
	// but infra-fleet-service additionally mounts "/agent" and
	// "/api/agent-token" on it for the Dev Server Agent, see that
	// service's cmd/server/main.go). Distinct from
	// OtherServiceAddrs["infra-fleet-service"], which is its gRPC address
	// (port 9090) used for the REST->gRPC routes in
	// internal/adapter/httpgateway/infra_routes.go. Empty disables the
	// /agent + /api/agent-token proxy routes (main.go degrades the same
	// way every other optional downstream does) rather than proxying to
	// an empty host.
	InfraFleetHTTPAddr string

	// RateLimitRPS/RateLimitBurst configure the per-tenant in-memory
	// token-bucket rate limiter (internal/usecase/rate_limit.go).
	RateLimitRPS   float64
	RateLimitBurst int

	// OtherServiceAddrs holds the remaining 14 downstream services'
	// addresses (auth, tenant, project, infra-fleet, git-gateway,
	// scm-integration, issue-tracking, ai-provider, workflow, task,
	// orchestration, automation, annotation, credential-broker). None of
	// these are dialed yet — every route under their prefix returns a 501
	// stub (internal/adapter/httpgateway) until each service's gRPC
	// contract stabilizes. Kept as a map rather than one field per service
	// so adding a real client later is a one-line change in main.go, not a
	// config struct edit.
	OtherServiceAddrs map[string]string
}

// Load reads api-gateway's configuration from the environment.
func Load() (Config, error) {
	base, err := commonconfig.LoadBase("api-gateway")
	if err != nil {
		return Config{}, err
	}

	publicPort, err := intEnv("PUBLIC_PORT", 8081)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Base:                    base,
		PublicPort:              publicPort,
		UsageServiceAddr:        commonconfig.StringEnv("USAGE_SERVICE_ADDR", "localhost:9101"),
		NotificationServiceAddr: commonconfig.StringEnv("NOTIFICATION_SERVICE_ADDR", "localhost:9102"),
		InfraFleetHTTPAddr:      commonconfig.StringEnv("INFRA_FLEET_SERVICE_HTTP_ADDR", ""),
		RateLimitRPS:            50,
		RateLimitBurst:          100,
		OtherServiceAddrs: map[string]string{
			"auth-service":              commonconfig.StringEnv("AUTH_SERVICE_ADDR", ""),
			"tenant-service":            commonconfig.StringEnv("TENANT_SERVICE_ADDR", ""),
			"project-service":           commonconfig.StringEnv("PROJECT_SERVICE_ADDR", ""),
			"infra-fleet-service":       commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", ""),
			"git-gateway-service":       commonconfig.StringEnv("GIT_GATEWAY_SERVICE_ADDR", ""),
			"scm-integration-service":   commonconfig.StringEnv("SCM_INTEGRATION_SERVICE_ADDR", ""),
			"issue-tracking-service":    commonconfig.StringEnv("ISSUE_TRACKING_SERVICE_ADDR", ""),
			"ai-provider-service":       commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", ""),
			"workflow-service":          commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", ""),
			"task-service":              commonconfig.StringEnv("TASK_SERVICE_ADDR", ""),
			"orchestration-service":     commonconfig.StringEnv("ORCHESTRATION_SERVICE_ADDR", ""),
			"automation-service":        commonconfig.StringEnv("AUTOMATION_SERVICE_ADDR", ""),
			"annotation-service":        commonconfig.StringEnv("ANNOTATION_SERVICE_ADDR", ""),
			"credential-broker-service": commonconfig.StringEnv("CREDENTIAL_BROKER_SERVICE_ADDR", ""),
		},
	}, nil
}

func intEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s=%q: %w", key, v, err)
	}
	return n, nil
}
