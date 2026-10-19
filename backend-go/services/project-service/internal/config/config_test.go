package config

import "testing"

func TestLoad_AllowLocalRepos(t *testing.T) {
	t.Setenv("ORCA_ALLOW_LOCAL_REPOS", "")
	cfg, err := Load()
	if err != nil || cfg.AllowLocalRepos {
		t.Fatalf("default must be off, got %v err=%v", cfg.AllowLocalRepos, err)
	}
	t.Setenv("ORCA_ALLOW_LOCAL_REPOS", "true")
	cfg, err = Load()
	if err != nil || !cfg.AllowLocalRepos {
		t.Fatalf("expected on, got %v err=%v", cfg.AllowLocalRepos, err)
	}
	t.Setenv("ORCA_ALLOW_LOCAL_REPOS", "maybe")
	if _, err = Load(); err == nil {
		t.Fatal("expected an error for an invalid bool")
	}
}
