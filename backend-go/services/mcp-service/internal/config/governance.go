package config

import (
	"fmt"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// GovernanceConfig configures the tool gate, approvals, kill switch and limiter
// (BE-MCP-SOL-012/013). Env names: OPA_BUNDLE_PATH, MCP_MAX_DEPTH,
// MCP_POLICY_CACHE_TTL, MCP_KILLSWITCH_POLL, MCP_TAINT_TTL,
// MCP_TOOL_CALL_MAX_SECONDS, MCP_TOOL_CALLS_RETENTION, MCP_RATE_*,
// MCP_MAX_PENDING_APPROVALS, MCP_MAX_APPROVALS_PER_HOUR, MCP_INTERNAL_CALLER_TOKEN.
type GovernanceConfig struct {
	BundlePath          string
	Usecase             usecase.GovernanceConfig
	ToolCallsRetention  time.Duration
	InternalCallerToken string // guards gateway-only RPCs when set
	WorkerInterval      time.Duration
}

func LoadGovernance() (GovernanceConfig, error) {
	uc := usecase.DefaultGovernanceConfig()
	var err error
	if uc.MaxDepth, err = intEnv("MCP_MAX_DEPTH", uc.MaxDepth); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.PolicyCacheTTL, err = durationEnv("MCP_POLICY_CACHE_TTL", uc.PolicyCacheTTL); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.KillStateTTL, err = durationEnv("MCP_KILLSWITCH_POLL", uc.KillStateTTL); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.TaintTTL, err = durationEnv("MCP_TAINT_TTL", uc.TaintTTL); err != nil {
		return GovernanceConfig{}, err
	}
	secs, err := intEnv("MCP_TOOL_CALL_MAX_SECONDS", int(uc.ToolCallMaxAge/time.Second))
	if err != nil {
		return GovernanceConfig{}, err
	}
	uc.ToolCallMaxAge = time.Duration(secs) * time.Second
	for env, key := range map[string]string{"MCP_RATE_EXEC_PER_MIN": "exec", "MCP_RATE_WRITE_PER_MIN": "write", "MCP_RATE_READ_PER_MIN": "read"} {
		if uc.Rate.PerMinute[key], err = intEnv(env, uc.Rate.PerMinute[key]); err != nil {
			return GovernanceConfig{}, err
		}
	}
	if uc.Rate.TotalPerMinute, err = intEnv("MCP_RATE_TOTAL_PER_MIN", uc.Rate.TotalPerMinute); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.Rate.DailyPerUser, err = intEnv("MCP_RATE_DAILY_PER_USER", uc.Rate.DailyPerUser); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.MaxPendingPerClient, err = intEnv("MCP_MAX_PENDING_APPROVALS", uc.MaxPendingPerClient); err != nil {
		return GovernanceConfig{}, err
	}
	if uc.MaxApprovalsPerHour, err = intEnv("MCP_MAX_APPROVALS_PER_HOUR", uc.MaxApprovalsPerHour); err != nil {
		return GovernanceConfig{}, err
	}
	retention, err := durationEnv("MCP_TOOL_CALLS_RETENTION", 30*24*time.Hour)
	if err != nil {
		return GovernanceConfig{}, err
	}
	cfg := GovernanceConfig{
		BundlePath:          commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
		Usecase:             uc,
		ToolCallsRetention:  retention,
		InternalCallerToken: commonconfig.StringEnv("MCP_INTERNAL_CALLER_TOKEN", ""),
		WorkerInterval:      10 * time.Second,
	}
	return cfg, cfg.Validate()
}

func (c GovernanceConfig) Validate() error {
	u := c.Usecase
	switch {
	case u.MaxDepth < 0:
		return fmt.Errorf("config: MCP_MAX_DEPTH must be >= 0")
	case u.PolicyCacheTTL <= 0 || u.KillStateTTL <= 0:
		return fmt.Errorf("config: MCP_POLICY_CACHE_TTL and MCP_KILLSWITCH_POLL must be positive")
	case u.KillStateTTL > 30*time.Second:
		// Kill switch must take effect within 60s even with the event bus down.
		return fmt.Errorf("config: MCP_KILLSWITCH_POLL must be <= 30s")
	case u.ToolCallMaxAge < time.Minute:
		return fmt.Errorf("config: MCP_TOOL_CALL_MAX_SECONDS must be >= 60")
	}
	return nil
}
