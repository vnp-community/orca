package config

import commonconfig "github.com/stablyai/orca-go/common/config"

// DirectoryCallerToken guards ListActiveTenantAdmins. It is read apart from OAuthConfig because that block is only
// loaded when the OAuth server is on, while request-service needs this RPC regardless. Empty refuses every call.
func DirectoryCallerToken() string {
	return commonconfig.StringEnv("AUTH_DIRECTORY_CALLER_TOKEN", "")
}
