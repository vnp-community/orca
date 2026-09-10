package localdaemon

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultPidFile returns BR-CLI-10's default pidfile location —
// ~/.local/share/orca/daemon.pid on Linux, matching the XDG Base
// Directory spec's data-home convention. There is no os.UserDataDir in
// the stdlib (unlike os.UserConfigDir/os.UserCacheDir), so userDataDir
// below fills that gap per-OS rather than hardcoding a Linux-only path,
// per AGENTS.md's file-paths rule.
func DefaultPidFile() (string, error) {
	dataHome, err := userDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataHome, "orca", "daemon.pid"), nil
}

// DefaultComposeFile resolves the ORCA_COMPOSE_FILE env override, falling
// back to "docker-compose.yml" relative to the working directory — the
// command layer runs this from wherever the user invoked orca-cli, same
// as `docker compose`'s own default file-discovery convention.
func DefaultComposeFile() string {
	if v := os.Getenv("ORCA_COMPOSE_FILE"); v != "" {
		return v
	}
	return "docker-compose.yml"
}

func userDataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return v, nil
		}
		return os.UserConfigDir()
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}
