package secrets

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	vault "github.com/hashicorp/vault/api"
)

// Sentinels let callers tell "policy missing" from "Vault down" without
// importing the Vault SDK.
var (
	ErrVaultForbidden   = errors.New("secrets: vault denied the request (HTTP 403)")
	ErrVaultUnavailable = errors.New("secrets: vault unavailable")
)

// TransitKeyInfo is the non-secret description of a Transit key.
type TransitKeyInfo struct {
	Type          string
	LatestVersion int
	// PublicKeyPEM is the latest version's public key; empty for symmetric types.
	PublicKeyPEM string
}

// classifyVaultError wraps err with ErrVaultForbidden / ErrVaultUnavailable
// when its HTTP status or transport nature says so.
func classifyVaultError(op string, err error) error {
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		switch {
		case respErr.StatusCode == http.StatusForbidden:
			return fmt.Errorf("%s: %w: %v", op, ErrVaultForbidden, err)
		case respErr.StatusCode >= 500:
			return fmt.Errorf("%s: %w: %v", op, ErrVaultUnavailable, err)
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", op, err)
	}
	// A non-HTTP error from the Vault client is a transport failure.
	return fmt.Errorf("%s: %w: %v", op, ErrVaultUnavailable, err)
}

// TransitReadKey describes the named Transit key, or (nil, nil) when it does
// not exist (Vault answers 404).
func (c *Client) TransitReadKey(ctx context.Context, keyName string) (*TransitKeyInfo, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "transit/keys/"+keyName)
	if err != nil {
		return nil, classifyVaultError("secrets: transit read key "+keyName, err)
	}
	if secret == nil {
		return nil, nil
	}
	info := &TransitKeyInfo{}
	info.Type, _ = secret.Data["type"].(string)
	if v, ok := secret.Data["latest_version"]; ok {
		n, convErr := intFromVaultNumber(v)
		if convErr != nil {
			return nil, fmt.Errorf("secrets: transit key %s: invalid latest_version: %w", keyName, convErr)
		}
		info.LatestVersion = n
	}
	if keys, ok := secret.Data["keys"].(map[string]any); ok {
		if entry, ok := keys[fmt.Sprint(info.LatestVersion)].(map[string]any); ok {
			info.PublicKeyPEM, _ = entry["public_key"].(string)
		}
	}
	return info, nil
}

// TransitCreateKey creates a Transit key of keyType. Vault treats creating an
// existing key as a no-op, so concurrent creators are safe. It never deletes
// or alters an existing key.
func (c *Client) TransitCreateKey(ctx context.Context, keyName, keyType string) error {
	if _, err := c.api.Logical().WriteWithContext(ctx, "transit/keys/"+keyName, map[string]any{"type": keyType}); err != nil {
		return classifyVaultError("secrets: transit create key "+keyName, err)
	}
	return nil
}
