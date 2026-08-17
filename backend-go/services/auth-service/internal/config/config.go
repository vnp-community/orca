// Package config loads auth-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// BcryptCost is the bcrypt work factor CreateUser hashes new
	// passwords with. Floored at adapter/bcrypt.MinCost (12) regardless of
	// what's configured — see auth-service.md §9.
	BcryptCost int
	// SessionTTL is how long a session created by Login stays valid.
	SessionTTL time.Duration
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("auth-service")
	if err != nil {
		return Config{}, err
	}

	bcryptCost, err := intEnv("BCRYPT_COST", 12)
	if err != nil {
		return Config{}, err
	}

	sessionTTL, err := durationEnv("SESSION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Base:       base,
		BcryptCost: bcryptCost,
		SessionTTL: sessionTTL,
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

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}
