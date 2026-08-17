// Package config loads task-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// OPABundlePath points ResolvePermission's OPA client
	// (internal/adapter/opaclient, via common/policy.Evaluator) at the
	// orca-authz Rego bundle on disk. Defaults to the bundle's location
	// relative to this service's module root when run the way this
	// service's README's "Running locally" section runs it (`cd
	// services/task-service && go run ./cmd/server`); override for
	// container images that lay the bundle out elsewhere.
	OPABundlePath string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("task-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:          base,
		OPABundlePath: commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
	}, nil
}
