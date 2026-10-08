package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	DatabaseCredentialsFile  string
	NATSURL                  string
	TaskServiceAddr          string
	AIProviderServiceAddr    string
	ProjectServiceAddr       string
	InfraFleetServiceAddr    string
	IssueTrackingServiceAddr string
	// TenantServiceAddr (teams) and AuthServiceAddr (admin list) feed approver and recipient lookups;
	// when unset those lookups fail closed instead of guessing.
	TenantServiceAddr     string
	AuthServiceAddr       string
	GitGatewayServiceAddr string
	// WebhookSourcesJSON configures POST /v1/request-webhooks/{source}; empty disables the route.
	WebhookSourcesJSON        string
	RequestFlowEnabled        bool
	AllowNoopApprovalHandlers bool
	// ApprovalEnabled serves ApprovalService and runs the expiry/reminder sweeper; on by default now that every subject has a real handler.
	ApprovalEnabled bool
	// ApprovalSweepInterval is the expire/remind tick (REQUEST_APPROVAL_SWEEP_INTERVAL, Go duration).
	ApprovalSweepInterval time.Duration
	// ApprovalNotifyMaxRecipients caps recipients per approval notification (REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS).
	ApprovalNotifyMaxRecipients int
	// ApprovalSubjects limits which subject types need a handler; empty means all.
	ApprovalSubjects []string
	// Analysis tunables (CR-REQ-007/008); zero values mean the defaults in usecase.DefaultAnalysisSettings.
	AICompleteTimeout          time.Duration
	AnalysisLeaseTTL           time.Duration
	AnalysisHeartbeat          time.Duration
	AnalysisRecoveryInterval   time.Duration
	AgentReadonlyTimeoutMS     int
	AgentReadonlyMaxPerProject int
	// AgentReadonlyUseAgentFlag defaults to true: set false to force prompt-only runs on every agent.
	AgentReadonlyUseAgentFlag bool
	RequireEnforcedReadonly   bool
	GatewayInternalToken      string
	ServiceInternalToken      string
	OPABundlePath             string
	// StuckThresholds says how long a Request may sit in a status before orca_request_stuck counts it
	// (REQUEST_STUCK_THRESHOLD_<STATUS>, Go duration). Proposed values, not yet tuned on real traffic.
	StuckThresholds map[string]time.Duration
	// MetricsSampleInterval is the refresh period of the database gauges (REQUEST_METRICS_SAMPLE_INTERVAL).
	MetricsSampleInterval time.Duration
	// EraseHMACKey derives the pseudonym that replaces reporter_id on erasure (REQUEST_ERASE_HMAC_KEY);
	// without it retention and EraseRequest refuse to run.
	EraseHMACKey string
	// WebhookRequireTimestamp refuses webhook calls without a fresh X-Orca-Timestamp (default true);
	// false is the migration window for senders that sign only the body.
	WebhookRequireTimestamp bool
	// RetentionRunAt is the UTC time of day of the daily retention run, "HH:MM".
	RetentionRunAt string
	// Execution loop (CR-REQ-013): REQUEST_MAX_PARALLEL_TASKS, REQUEST_MAX_TASK_ATTEMPTS, REQUEST_AUTO_COMPLETE_TASKS,
	// REQUEST_DISPATCH_RETRY_WINDOW, REQUEST_RECONCILE_INTERVAL, REQUEST_RECONCILE_QUIET. Invalid values keep the default.
	MaxParallelTasks    int
	MaxTaskAttempts     int
	AutoCompleteTasks   bool
	DispatchRetryWindow time.Duration
	ReconcileInterval   time.Duration
	ReconcileQuiet      time.Duration
}

// stuckStatuses are the statuses a Request should leave without a human; waiting for people is not "stuck".
var stuckStatuses = []string{"classifying", "analyzing", "planning", "executing"}

func Load() Config {
	base, _ := commonconfig.LoadBase("request-service")
	c := Config{
		Base:                        base,
		ApprovalEnabled:             true,
		ApprovalSweepInterval:       time.Minute,
		ApprovalNotifyMaxRecipients: 50,
		AgentReadonlyUseAgentFlag:   true,
		OPABundlePath:               "../../policy/orca-authz",
		WebhookRequireTimestamp:     true,
		RetentionRunAt:              "03:00",
		MaxParallelTasks:            1,
		MaxTaskAttempts:             2,
		AutoCompleteTasks:           true,
		DispatchRetryWindow:         15 * time.Minute,
		ReconcileInterval:           time.Minute,
		ReconcileQuiet:              5 * time.Minute,
		MetricsSampleInterval:       30 * time.Second,
		StuckThresholds:             map[string]time.Duration{"classifying": 10 * time.Minute, "analyzing": 30 * time.Minute, "planning": 30 * time.Minute},
	}
	for _, status := range stuckStatuses {
		if v := os.Getenv("REQUEST_STUCK_THRESHOLD_" + strings.ToUpper(status)); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				c.StuckThresholds[status] = d
			}
		}
	}
	if v := os.Getenv("REQUEST_METRICS_SAMPLE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.MetricsSampleInterval = d
		}
	}
	c.loadExecution()

	if f := os.Getenv("DATABASE_CREDENTIALS_FILE"); f != "" {
		c.DatabaseCredentialsFile = f
	}
	if u := os.Getenv("NATS_URL"); u != "" {
		c.NATSURL = u
	}
	if a := os.Getenv("TASK_SERVICE_ADDR"); a != "" {
		c.TaskServiceAddr = a
	}
	if a := os.Getenv("AI_PROVIDER_SERVICE_ADDR"); a != "" {
		c.AIProviderServiceAddr = a
	}
	if a := os.Getenv("PROJECT_SERVICE_ADDR"); a != "" {
		c.ProjectServiceAddr = a
	}
	if a := os.Getenv("INFRA_FLEET_SERVICE_ADDR"); a != "" {
		c.InfraFleetServiceAddr = a
	}
	if a := os.Getenv("TENANT_SERVICE_ADDR"); a != "" {
		c.TenantServiceAddr = a
	}
	if a := os.Getenv("AUTH_SERVICE_ADDR"); a != "" {
		c.AuthServiceAddr = a
	}
	if s := os.Getenv("REQUEST_APPROVAL_SWEEP_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			c.ApprovalSweepInterval = d
		}
	}
	if s := os.Getenv("REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			c.ApprovalNotifyMaxRecipients = n
		}
	}
	if a := os.Getenv("ISSUE_TRACKING_SERVICE_ADDR"); a != "" {
		c.IssueTrackingServiceAddr = a
	}
	if a := os.Getenv("GIT_GATEWAY_SERVICE_ADDR"); a != "" {
		c.GitGatewayServiceAddr = a
	}
	c.AICompleteTimeout = envDuration("REQUEST_AI_COMPLETE_TIMEOUT")
	c.AnalysisLeaseTTL = envDuration("REQUEST_ANALYSIS_LEASE_TTL")
	c.AnalysisHeartbeat = envDuration("REQUEST_ANALYSIS_HEARTBEAT")
	c.AnalysisRecoveryInterval = envDuration("REQUEST_ANALYSIS_RECOVERY_INTERVAL")
	c.AgentReadonlyTimeoutMS = envPositiveInt("REQUEST_AGENT_READONLY_TIMEOUT_MS")
	c.AgentReadonlyMaxPerProject = envPositiveInt("REQUEST_AGENT_READONLY_MAX_PER_PROJECT")
	if s := os.Getenv("REQUEST_AGENT_READONLY_USE_AGENT_FLAG"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.AgentReadonlyUseAgentFlag = b
		}
	}
	if s := os.Getenv("REQUEST_REQUIRE_ENFORCED_READONLY"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.RequireEnforcedReadonly = b
		}
	}
	c.WebhookSourcesJSON = os.Getenv("REQUEST_WEBHOOK_SOURCES")
	if s := os.Getenv("REQUEST_FLOW_ENABLED"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.RequestFlowEnabled = b
		}
	}
	if s := os.Getenv("REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.AllowNoopApprovalHandlers = b
		}
	}
	if s := os.Getenv("REQUEST_APPROVAL_ENABLED"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.ApprovalEnabled = b
		}
	}
	for _, part := range strings.Split(os.Getenv("REQUEST_APPROVAL_SUBJECTS"), ",") {
		if part = strings.TrimSpace(part); part != "" {
			c.ApprovalSubjects = append(c.ApprovalSubjects, part)
		}
	}
	if tok := os.Getenv("GATEWAY_INTERNAL_TOKEN"); tok != "" {
		c.GatewayInternalToken = tok
	}
	if tok := os.Getenv("SERVICE_INTERNAL_TOKEN"); tok != "" {
		c.ServiceInternalToken = tok
	}
	if p := os.Getenv("OPA_BUNDLE_PATH"); p != "" {
		c.OPABundlePath = p
	}

	c.EraseHMACKey = os.Getenv("REQUEST_ERASE_HMAC_KEY")
	if s := os.Getenv("REQUEST_WEBHOOK_REQUIRE_TIMESTAMP"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			c.WebhookRequireTimestamp = b
		}
	}
	if s := os.Getenv("REQUEST_RETENTION_RUN_AT"); s != "" {
		c.RetentionRunAt = s
	}

	return c
}

// envDuration reads a Go duration such as "90s"; a missing or invalid value yields 0, which selects the default.
func envDuration(key string) time.Duration {
	d, err := time.ParseDuration(os.Getenv(key))
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

func envPositiveInt(key string) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// loadExecution reads the execution-loop settings; a value that does not parse or is out of range is ignored.
func (c *Config) loadExecution() {
	intIn := func(key string, lo, hi int, dst *int) {
		if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n >= lo && n <= hi {
			*dst = n
		}
	}
	durationAbove := func(key string, dst *time.Duration) {
		if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d > 0 {
			*dst = d
		}
	}
	intIn("REQUEST_MAX_PARALLEL_TASKS", 1, 10, &c.MaxParallelTasks)
	intIn("REQUEST_MAX_TASK_ATTEMPTS", 1, 10, &c.MaxTaskAttempts)
	if b, err := strconv.ParseBool(os.Getenv("REQUEST_AUTO_COMPLETE_TASKS")); err == nil {
		c.AutoCompleteTasks = b
	}
	durationAbove("REQUEST_DISPATCH_RETRY_WINDOW", &c.DispatchRetryWindow)
	durationAbove("REQUEST_RECONCILE_INTERVAL", &c.ReconcileInterval)
	durationAbove("REQUEST_RECONCILE_QUIET", &c.ReconcileQuiet)
}
