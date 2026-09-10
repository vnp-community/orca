// Package config loads notification-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	NATSURL string
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for real by internal/adapter/vaultsigner as of Epic B
	// (docs/execution-plan.md §8), which replaced this service's previous
	// direct common/secrets.TransitEncrypt call with a
	// credentialbrokerv1.SignVapidPayload RPC, closing the one documented
	// exception to "no service but credential-broker-service touches
	// Vault directly."
	CredentialBrokerAddr string
	// AuthServiceAddr is auth-service's gRPC target — dialed by
	// internal/adapter/grpcclient/authclient.DeviceSecretResolver to call
	// ResolveDeviceSharedSecret (SOL-MB-01), an internal-only RPC never
	// routed through api-gateway's REST facade (BL-MB-02).
	AuthServiceAddr string
	// APNsTeamID/APNsKeyID/APNsTopic/APNsEndpoint configure the iOS push
	// channel's provider-token auth (BL-MB-02/TASK-MB-02-08) — none of
	// these are secret material themselves (Apple assigns them as public
	// identifiers); the actual .p8 signing key lives only in Vault
	// Transit, never in this process. Left unset in an environment with no
	// real APNs credentials provisioned — apns.Client.Send then returns a
	// clear config error instead of attempting a call.
	APNsTeamID   string
	APNsKeyID    string
	APNsTopic    string
	APNsEndpoint string
	// FCMProjectID/FCMServiceAccountEmail configure the Android push
	// channel's OAuth2 service-account auth — same "left unset until real
	// credentials are provisioned" posture as APNs above.
	FCMProjectID           string
	FCMServiceAccountEmail string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("notification-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                   base,
		NATSURL:                commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		CredentialBrokerAddr:   commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		AuthServiceAddr:        commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
		APNsTeamID:             commonconfig.StringEnv("APNS_TEAM_ID", ""),
		APNsKeyID:              commonconfig.StringEnv("APNS_KEY_ID", ""),
		APNsTopic:              commonconfig.StringEnv("APNS_TOPIC", ""),
		APNsEndpoint:           commonconfig.StringEnv("APNS_ENDPOINT", "https://api.push.apple.com"),
		FCMProjectID:           commonconfig.StringEnv("FCM_PROJECT_ID", ""),
		FCMServiceAccountEmail: commonconfig.StringEnv("FCM_SERVICE_ACCOUNT_EMAIL", ""),
	}, nil
}
