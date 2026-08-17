// Package secrets wraps the HashiCorp Vault client with the specific
// operations services in this system actually need, per
// specs/backend-go/architecture/06-secrets-vault-architecture.md. Every
// service uses DatabaseCredentials (via the Vault Agent sidecar file, in
// production — see below); only credential-broker-service uses the
// Transit/KV/SSH methods directly, per that doc's "no other service talks
// to Vault directly for tenant secret material" rule.
package secrets

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

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
