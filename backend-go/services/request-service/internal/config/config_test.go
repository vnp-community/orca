package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	c := Load()
	// commonconfig.LoadBase() typical defaults if nothing is set
	if c.DatabaseCredentialsFile != "" {
		t.Errorf("expected empty DatabaseCredentialsFile, got %q", c.DatabaseCredentialsFile)
	}
	if c.NATSURL != "" {
		t.Errorf("expected empty NATSURL, got %q", c.NATSURL)
	}
	if c.TaskServiceAddr != "" {
		t.Errorf("expected empty TaskServiceAddr, got %q", c.TaskServiceAddr)
	}
	if c.AIProviderServiceAddr != "" {
		t.Errorf("expected empty AIProviderServiceAddr, got %q", c.AIProviderServiceAddr)
	}
	if c.ProjectServiceAddr != "" {
		t.Errorf("expected empty ProjectServiceAddr, got %q", c.ProjectServiceAddr)
	}
	if c.RequestFlowEnabled {
		t.Errorf("expected RequestFlowEnabled to be false by default")
	}
}

func TestLoad_Overrides(t *testing.T) {
	os.Setenv("DATABASE_CREDENTIALS_FILE", "/path/to/creds")
	defer os.Unsetenv("DATABASE_CREDENTIALS_FILE")

	os.Setenv("NATS_URL", "nats://localhost:4222")
	defer os.Unsetenv("NATS_URL")

	os.Setenv("TASK_SERVICE_ADDR", "localhost:9091")
	defer os.Unsetenv("TASK_SERVICE_ADDR")

	os.Setenv("AI_PROVIDER_SERVICE_ADDR", "localhost:9092")
	defer os.Unsetenv("AI_PROVIDER_SERVICE_ADDR")

	os.Setenv("PROJECT_SERVICE_ADDR", "localhost:9093")
	defer os.Unsetenv("PROJECT_SERVICE_ADDR")

	os.Setenv("REQUEST_FLOW_ENABLED", "true")
	defer os.Unsetenv("REQUEST_FLOW_ENABLED")

	c := Load()
	if c.DatabaseCredentialsFile != "/path/to/creds" {
		t.Errorf("got %q", c.DatabaseCredentialsFile)
	}
	if c.NATSURL != "nats://localhost:4222" {
		t.Errorf("got %q", c.NATSURL)
	}
	if c.TaskServiceAddr != "localhost:9091" {
		t.Errorf("got %q", c.TaskServiceAddr)
	}
	if c.AIProviderServiceAddr != "localhost:9092" {
		t.Errorf("got %q", c.AIProviderServiceAddr)
	}
	if c.ProjectServiceAddr != "localhost:9093" {
		t.Errorf("got %q", c.ProjectServiceAddr)
	}
	if !c.RequestFlowEnabled {
		t.Errorf("expected RequestFlowEnabled to be true")
	}
}

func TestLoad_InvalidBool(t *testing.T) {
	// If the bool cannot be parsed, it should default to false.
	// This matches the design of falling back on false when error occurs.
	os.Setenv("REQUEST_FLOW_ENABLED", "maybe")
	defer os.Unsetenv("REQUEST_FLOW_ENABLED")

	c := Load()
	if c.RequestFlowEnabled {
		t.Errorf("expected RequestFlowEnabled to be false when given invalid string")
	}
}

func TestLoad_ApprovalDefaults(t *testing.T) {
	c := Load()
	if !c.ApprovalEnabled || c.AllowNoopApprovalHandlers || len(c.ApprovalSubjects) != 0 {
		t.Errorf("approval is on with real handlers only (noop never by default): %+v", c)
	}
	if c.ApprovalSweepInterval != time.Minute || c.ApprovalNotifyMaxRecipients != 50 {
		t.Errorf("sweep/notify defaults wrong: %v %d", c.ApprovalSweepInterval, c.ApprovalNotifyMaxRecipients)
	}
}

func TestLoad_ApprovalSweepAndNotifyOverrides(t *testing.T) {
	t.Setenv("REQUEST_APPROVAL_SWEEP_INTERVAL", "15s")
	t.Setenv("REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS", "7")
	t.Setenv("TENANT_SERVICE_ADDR", "tenant:9090")
	t.Setenv("AUTH_SERVICE_ADDR", "auth:9090")
	c := Load()
	if c.ApprovalSweepInterval != 15*time.Second || c.ApprovalNotifyMaxRecipients != 7 || c.TenantServiceAddr != "tenant:9090" || c.AuthServiceAddr != "auth:9090" {
		t.Errorf("overrides not read: %+v", c)
	}
	t.Setenv("REQUEST_APPROVAL_SWEEP_INTERVAL", "garbage")
	t.Setenv("REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS", "-3")
	if c := Load(); c.ApprovalSweepInterval != time.Minute || c.ApprovalNotifyMaxRecipients != 50 {
		t.Errorf("invalid values must keep defaults: %+v", c)
	}
}

func TestLoad_ApprovalOverrides(t *testing.T) {
	t.Setenv("REQUEST_APPROVAL_ENABLED", "true")
	t.Setenv("REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS", "true")
	t.Setenv("REQUEST_APPROVAL_SUBJECTS", " solution, plan ,,")
	c := Load()
	if !c.ApprovalEnabled || !c.AllowNoopApprovalHandlers {
		t.Errorf("flags not read: %+v", c)
	}
	if len(c.ApprovalSubjects) != 2 || c.ApprovalSubjects[0] != "solution" || c.ApprovalSubjects[1] != "plan" {
		t.Errorf("subjects = %v", c.ApprovalSubjects)
	}
}

func TestLoad_AnalysisSettings(t *testing.T) {
	c := Load()
	if c.AICompleteTimeout != 0 || c.AnalysisLeaseTTL != 0 || c.AgentReadonlyMaxPerProject != 0 || c.RequireEnforcedReadonly || !c.AgentReadonlyUseAgentFlag {
		t.Fatalf("defaults leaked into config (zero means use the usecase default): %+v", c)
	}

	t.Setenv("REQUEST_AI_COMPLETE_TIMEOUT", "45s")
	t.Setenv("REQUEST_ANALYSIS_LEASE_TTL", "2m")
	t.Setenv("REQUEST_ANALYSIS_HEARTBEAT", "10s")
	t.Setenv("REQUEST_ANALYSIS_RECOVERY_INTERVAL", "5s")
	t.Setenv("REQUEST_AGENT_READONLY_TIMEOUT_MS", "120000")
	t.Setenv("REQUEST_AGENT_READONLY_MAX_PER_PROJECT", "3")
	t.Setenv("REQUEST_AGENT_READONLY_USE_AGENT_FLAG", "false")
	t.Setenv("REQUEST_REQUIRE_ENFORCED_READONLY", "true")
	t.Setenv("GIT_GATEWAY_SERVICE_ADDR", "git:9000")
	c = Load()
	if c.AICompleteTimeout != 45*time.Second || c.AnalysisLeaseTTL != 2*time.Minute || c.AnalysisHeartbeat != 10*time.Second || c.AnalysisRecoveryInterval != 5*time.Second {
		t.Fatalf("durations = %v %v %v %v", c.AICompleteTimeout, c.AnalysisLeaseTTL, c.AnalysisHeartbeat, c.AnalysisRecoveryInterval)
	}
	if c.AgentReadonlyTimeoutMS != 120000 || c.AgentReadonlyMaxPerProject != 3 || c.AgentReadonlyUseAgentFlag || !c.RequireEnforcedReadonly || c.GitGatewayServiceAddr != "git:9000" {
		t.Fatalf("config = %+v", c)
	}

	t.Setenv("REQUEST_AI_COMPLETE_TIMEOUT", "soon")
	t.Setenv("REQUEST_AGENT_READONLY_MAX_PER_PROJECT", "-4")
	c = Load()
	if c.AICompleteTimeout != 0 || c.AgentReadonlyMaxPerProject != 0 {
		t.Fatalf("invalid values must fall back to the default, got %v and %d", c.AICompleteTimeout, c.AgentReadonlyMaxPerProject)
	}
}
