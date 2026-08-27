package sshrelay

import (
	"os"
	"time"
)

// Config configures the deploy/launch/handshake pipeline.
type Config struct {
	// BundlePath is the local filesystem path to the built agent bundle
	// (agent/out/agent.js) this service SFTP-deploys to each relay-ssh
	// target. Empty means relay-ssh deploy has nothing to deploy — Deploy
	// returns a clear error rather than silently no-op'ing.
	BundlePath string
	// HandshakeTimeout bounds how long Provisioner waits for the launched
	// agent's first agent.handshake frame — mirrors
	// adapter/agentwsserver's identical 20s convention for the same wait,
	// direct-websocket's inbound counterpart.
	HandshakeTimeout time.Duration
	// OrcaVersion is echoed back in a successful agent.handshake response's
	// orcaVersion field — reused from the same ORCA_VERSION env var
	// devserveragent.Config already reads, threaded in via
	// LoadConfigFromEnv's parameter rather than a second os.Getenv call.
	OrcaVersion string
}

// LoadConfigFromEnv reads ORCA_RELAY_BUNDLE_PATH and combines it with
// orcaVersion supplied by the caller (main.go already computes it via
// devserveragent.LoadConfigFromEnv()).
func LoadConfigFromEnv(orcaVersion string) Config {
	return Config{
		BundlePath:       os.Getenv("ORCA_RELAY_BUNDLE_PATH"),
		HandshakeTimeout: 20 * time.Second,
		OrcaVersion:      orcaVersion,
	}
}
