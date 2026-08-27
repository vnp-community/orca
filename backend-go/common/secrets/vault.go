// Package secrets wraps the HashiCorp Vault client with the specific
// operations services in this system actually need, per
// specs/backend-go/architecture/06-secrets-vault-architecture.md. Every
// service uses DatabaseCredentials (via the Vault Agent sidecar file, in
// production — see below). For tenant secret material, only
// credential-broker-service uses the Transit/KV/SSH methods directly, per
// that doc's "no other service talks to Vault directly for tenant secret
// material" rule. auth-service is the one other direct Transit caller
// (Epic D) — its JWT signing key is a service-wide signing identity, not
// tenant secret material, so it falls outside that rule.
package secrets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// Client wraps a Vault API client scoped to this service's own policy.
type Client struct {
	api *vault.Client
}

// NewClient builds a Client from the standard VAULT_ADDR/VAULT_TOKEN
// environment — local-dev convenience. Production services authenticate via
// the Kubernetes auth method through a Vault Agent sidecar instead of a
// static token; see architecture/06's auth-flow diagram. This constructor
// is what that sidecar's templated environment ultimately feeds into.
func NewClient() (*Client, error) {
	cfg := vault.DefaultConfig()
	if addr := os.Getenv("VAULT_ADDR"); addr != "" {
		cfg.Address = addr
	}
	c, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("secrets: creating vault client: %w", err)
	}
	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		c.SetToken(token)
	}
	return &Client{api: c}, nil
}

// DatabaseCredentialsFromFile reads dynamic Postgres credentials the Vault
// Agent sidecar has already fetched and rendered to disk (the pattern
// architecture/06 recommends over embedding token-refresh logic in
// application code). Falls back to the DATABASE_DSN env var when the file
// doesn't exist, which is what every service's local-dev/testcontainers
// path uses instead of a real Vault Agent.
func DatabaseCredentialsFromFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
				return dsn, nil
			}
		}
		return "", fmt.Errorf("secrets: reading rendered DB credentials: %w", err)
	}
	return string(b), nil
}

// TransitEncrypt encrypts plaintext under the named Transit key —
// credential-broker-service's primary mechanism for AI-provider-key and
// VAPID at-rest storage (see architecture/06). Returns Vault's ciphertext
// wire format ("vault:v1:...").
func (c *Client) TransitEncrypt(ctx context.Context, keyName string, plaintext []byte) (string, error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, "transit/encrypt/"+keyName, map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	})
	if err != nil {
		return "", fmt.Errorf("secrets: transit encrypt: %w", err)
	}
	ct, _ := secret.Data["ciphertext"].(string)
	return ct, nil
}

// TransitDecrypt is the inverse of TransitEncrypt — plaintext exists in the
// caller's memory only for the duration of the single request that invoked
// this, per credential-broker-service.md's guarantee.
func (c *Client) TransitDecrypt(ctx context.Context, keyName, ciphertext string) ([]byte, error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, "transit/decrypt/"+keyName, map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return nil, fmt.Errorf("secrets: transit decrypt: %w", err)
	}
	encoded, _ := secret.Data["plaintext"].(string)
	return base64.StdEncoding.DecodeString(encoded)
}

// KVWrite writes a versioned secret to the given KV v2 path (OAuth tokens,
// SSH static keys, service-to-service shared secrets — see architecture/06).
func (c *Client) KVWrite(ctx context.Context, mount, path string, data map[string]any) error {
	_, err := c.api.Logical().WriteWithContext(ctx, fmt.Sprintf("%s/data/%s", mount, path), map[string]any{
		"data": data,
	})
	if err != nil {
		return fmt.Errorf("secrets: kv write %s/%s: %w", mount, path, err)
	}
	return nil
}

// KVRead reads the current version of a KV v2 secret.
func (c *Client) KVRead(ctx context.Context, mount, path string) (map[string]any, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, fmt.Sprintf("%s/data/%s", mount, path))
	if err != nil {
		return nil, fmt.Errorf("secrets: kv read %s/%s: %w", mount, path, err)
	}
	if secret == nil {
		return nil, fmt.Errorf("secrets: kv read %s/%s: not found", mount, path)
	}
	data, _ := secret.Data["data"].(map[string]any)
	return data, nil
}

// TransitEnsureKey creates the named Transit key if it doesn't already
// exist (e.g. "rsa-2048" for JWT signing — Epic D). Idempotent: Vault's
// create-key call is itself a no-op against an existing key of the same
// type, but this still reads first so a type mismatch on an
// already-existing key surfaces as a clear error instead of being silently
// accepted.
func (c *Client) TransitEnsureKey(ctx context.Context, keyName, keyType string) error {
	secret, err := c.api.Logical().ReadWithContext(ctx, "transit/keys/"+keyName)
	if err != nil {
		return fmt.Errorf("secrets: transit read key %s: %w", keyName, err)
	}
	if secret != nil {
		if existingType, _ := secret.Data["type"].(string); existingType != "" && existingType != keyType {
			return fmt.Errorf("secrets: transit key %s already exists with type %q, expected %q", keyName, existingType, keyType)
		}
		return nil
	}
	if _, err := c.api.Logical().WriteWithContext(ctx, "transit/keys/"+keyName, map[string]any{
		"type": keyType,
	}); err != nil {
		return fmt.Errorf("secrets: transit create key %s: %w", keyName, err)
	}
	return nil
}

// TransitSign signs input under the named Transit key's private key, which
// never leaves Vault — the caller gets back a signature, never the key
// material. Returns Vault's wire format ("vault:v<version>:<base64 sig>");
// callers that need the raw signature bytes (e.g. to embed in a JWT) must
// strip the "vault:v<N>:" prefix themselves.
func (c *Client) TransitSign(ctx context.Context, keyName string, input []byte) (string, error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, "transit/sign/"+keyName, map[string]any{
		"input":          base64.StdEncoding.EncodeToString(input),
		"hash_algorithm": "sha2-256",
	})
	if err != nil {
		return "", fmt.Errorf("secrets: transit sign: %w", err)
	}
	if secret == nil {
		return "", fmt.Errorf("secrets: transit sign %s: empty response", keyName)
	}
	sig, _ := secret.Data["signature"].(string)
	if sig == "" {
		return "", fmt.Errorf("secrets: transit sign %s: no signature in response", keyName)
	}
	return sig, nil
}

// TransitPublicKey returns the named Transit key's latest public key
// (PEM-encoded, for an asymmetric key type such as "rsa-2048") plus its
// version — needed to publish a JWKS document (Epic D). Vault never
// exposes the corresponding private key through this or any other read.
func (c *Client) TransitPublicKey(ctx context.Context, keyName string) (pemPublicKey string, keyVersion int, err error) {
	versions, latest, err := c.TransitPublicKeyVersions(ctx, keyName)
	if err != nil {
		return "", 0, err
	}
	pub, ok := versions[latest]
	if !ok {
		return "", 0, fmt.Errorf("secrets: transit key %s: missing version %d data", keyName, latest)
	}
	return pub, latest, nil
}

// TransitPublicKeyVersions returns every version's PEM-encoded public key
// still readable for the named asymmetric Transit key, plus the latest
// version number — a JWKS publisher needs more than just the latest
// version to honor the rotation-overlap window (auth-service.md §9: the
// previous key stays published until every JWT signed under it expires).
// Vault keeps historical asymmetric key versions readable indefinitely
// unless explicitly deleted/trimmed, so this reflects whatever the Transit
// engine currently reports — no separate expiry bookkeeping here.
func (c *Client) TransitPublicKeyVersions(ctx context.Context, keyName string) (versions map[int]string, latestVersion int, err error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "transit/keys/"+keyName)
	if err != nil {
		return nil, 0, fmt.Errorf("secrets: transit read key %s: %w", keyName, err)
	}
	if secret == nil {
		return nil, 0, fmt.Errorf("secrets: transit key %s: not found", keyName)
	}
	latestVersion, err = intFromVaultNumber(secret.Data["latest_version"])
	if err != nil {
		return nil, 0, fmt.Errorf("secrets: transit key %s: invalid latest_version: %w", keyName, err)
	}
	keys, ok := secret.Data["keys"].(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("secrets: transit key %s: missing keys data", keyName)
	}
	versions = make(map[int]string, len(keys))
	for versionStr, raw := range keys {
		version, convErr := strconv.Atoi(versionStr)
		if convErr != nil {
			continue
		}
		versionData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pub, _ := versionData["public_key"].(string)
		if strings.TrimSpace(pub) == "" {
			continue // symmetric or non-signing key type: no public half to publish
		}
		versions[version] = pub
	}
	if len(versions) == 0 {
		return nil, 0, fmt.Errorf("secrets: transit key %s: no public_key at any version (not an asymmetric key?)", keyName)
	}
	return versions, latestVersion, nil
}

// SSHSignPublicKey requests Vault's SSH secrets engine (ssh/sign/<role>) to
// sign publicKeyOpenSSH (an OpenSSH-format public key, e.g.
// "ssh-ed25519 AAAA... comment") into a short-lived certificate. Never
// signs or stores a private key — callers generate an ephemeral keypair,
// this only signs the public half. See
// specs/backend-go/services/infra-fleet-service.md §9's "Preferred: Vault's
// SSH secrets engine" model.
func (c *Client) SSHSignPublicKey(ctx context.Context, role, publicKeyOpenSSH string) (signedCertOpenSSH string, err error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, "ssh/sign/"+role, map[string]any{
		"public_key": publicKeyOpenSSH,
	})
	if err != nil {
		return "", fmt.Errorf("secrets: ssh sign %s: %w", role, err)
	}
	if secret == nil {
		return "", fmt.Errorf("secrets: ssh sign %s: empty response", role)
	}
	signed, _ := secret.Data["signed_key"].(string)
	if signed == "" {
		return "", fmt.Errorf("secrets: ssh sign %s: no signed_key in response", role)
	}
	return signed, nil
}

// Ping performs a real Vault reachability/health check via Sys().Health() —
// added so credential-broker-service.SecretStore.Ping (previously a
// well-known-absent-KV-path heuristic, see that package's doc comment) can
// use a proper native call instead. A sealed or uninitialized Vault still
// answers Health() (it reports that state in the response body rather than
// erroring), so this only errors on genuine unreachability/transport
// failure — callers that care about sealed/standby state should inspect a
// richer result; this method's contract is deliberately just
// "reachable or not."
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.api.Sys().HealthWithContext(ctx); err != nil {
		return fmt.Errorf("secrets: vault health check: %w", err)
	}
	return nil
}

// KVDestroyMetadata permanently deletes a KV v2 secret's metadata AND every
// version's data at mount/path (Vault's native "delete metadata" API,
// `DELETE <mount>/metadata/<path>`) — unlike KVWrite-with-empty-payload
// (which only adds a new, recoverable version), this actually scrubs prior
// versions from the storage backend. This is what
// credential-broker-service.SecretStore.RevokeSecret was documented as
// needing but couldn't do without this method — see that package's doc
// comment, now updated to call this instead of the overwrite workaround.
func (c *Client) KVDestroyMetadata(ctx context.Context, mount, path string) error {
	if _, err := c.api.Logical().DeleteWithContext(ctx, fmt.Sprintf("%s/metadata/%s", mount, path)); err != nil {
		return fmt.Errorf("secrets: kv destroy metadata %s/%s: %w", mount, path, err)
	}
	return nil
}

// intFromVaultNumber handles both encodings the Vault SDK's JSON decoding
// can hand back for a numeric field depending on call path (json.Number
// when decoded with UseNumber, float64 otherwise).
func intFromVaultNumber(v any) (int, error) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return int(i), err
	case float64:
		return int(n), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", v)
	}
}
