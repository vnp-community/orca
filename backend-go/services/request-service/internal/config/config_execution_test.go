package config

import (
	"testing"
	"time"
)

func TestLoad_ExecutionDefaults(t *testing.T) {
	c := Load()
	if c.MaxParallelTasks != 1 || c.MaxTaskAttempts != 2 || !c.AutoCompleteTasks || c.DispatchRetryWindow != 15*time.Minute ||
		c.ReconcileInterval != time.Minute || c.ReconcileQuiet != 5*time.Minute {
		t.Fatalf("defaults: %+v", c)
	}
}

func TestLoad_ExecutionOverrides(t *testing.T) {
	t.Setenv("REQUEST_MAX_PARALLEL_TASKS", "3")
	t.Setenv("REQUEST_MAX_TASK_ATTEMPTS", "4")
	t.Setenv("REQUEST_AUTO_COMPLETE_TASKS", "false")
	t.Setenv("REQUEST_DISPATCH_RETRY_WINDOW", "30m")
	t.Setenv("REQUEST_RECONCILE_INTERVAL", "10s")
	t.Setenv("REQUEST_RECONCILE_QUIET", "2m")
	c := Load()
	if c.MaxParallelTasks != 3 || c.MaxTaskAttempts != 4 || c.AutoCompleteTasks || c.DispatchRetryWindow != 30*time.Minute ||
		c.ReconcileInterval != 10*time.Second || c.ReconcileQuiet != 2*time.Minute {
		t.Fatalf("overrides: %+v", c)
	}
}

func TestLoad_ExecutionInvalidValuesKeepDefaults(t *testing.T) {
	for k, v := range map[string]string{
		"REQUEST_MAX_PARALLEL_TASKS": "0", "REQUEST_MAX_TASK_ATTEMPTS": "-2", "REQUEST_AUTO_COMPLETE_TASKS": "maybe",
		"REQUEST_DISPATCH_RETRY_WINDOW": "soon", "REQUEST_RECONCILE_INTERVAL": "-5s", "REQUEST_RECONCILE_QUIET": "0s",
	} {
		t.Setenv(k, v)
	}
	c := Load()
	if c.MaxParallelTasks != 1 || c.MaxTaskAttempts != 2 || !c.AutoCompleteTasks || c.DispatchRetryWindow != 15*time.Minute ||
		c.ReconcileInterval != time.Minute || c.ReconcileQuiet != 5*time.Minute {
		t.Fatalf("invalid values must fall back to the defaults: %+v", c)
	}
	t.Setenv("REQUEST_MAX_PARALLEL_TASKS", "11")
	if Load().MaxParallelTasks != 1 {
		t.Fatal("above the upper bound is invalid too")
	}
}
