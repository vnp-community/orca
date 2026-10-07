package config

import (
	"os"
	"testing"
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
