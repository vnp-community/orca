package config

import (
	"os"
	"strconv"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	DatabaseCredentialsFile string
	NATSURL                 string
	TaskServiceAddr         string
	AIProviderServiceAddr   string
	ProjectServiceAddr      string
	RequestFlowEnabled        bool
	AllowNoopApprovalHandlers bool
	GatewayInternalToken      string
	ServiceInternalToken      string
	OPABundlePath             string
}

func Load() Config {
	base, _ := commonconfig.LoadBase("request-service")
	c := Config{
		Base: base,
	}

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
	if tok := os.Getenv("GATEWAY_INTERNAL_TOKEN"); tok != "" {
		c.GatewayInternalToken = tok
	}
	if tok := os.Getenv("SERVICE_INTERNAL_TOKEN"); tok != "" {
		c.ServiceInternalToken = tok
	}
	if p := os.Getenv("OPA_BUNDLE_PATH"); p != "" {
		c.OPABundlePath = p
	}

	return c
}
