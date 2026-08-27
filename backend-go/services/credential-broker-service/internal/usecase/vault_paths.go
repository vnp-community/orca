package usecase

import "fmt"

// Hardcoded placeholder Vault mount/key names.
//
// KNOWN GAP (see this service's README): these are NOT the result of a real
// Vault policy design — a production deployment needs an actual KV v2 mount
// and per-category Transit keys provisioned by platform/security, with this
// service's Vault Kubernetes-auth role's policy scoped to exactly those
// names (credential-broker-service.md §9's least-privilege table). This
// scaffold picks fixed names so the rest of the flow — encrypt, store a
// pointer, audit — is real, working, and testable end to end against a real
// Vault dev server, without requiring that policy design to already exist.
const (
	// kvMount is the KV v2 mount every category's ciphertext pointer is
	// written under.
	kvMount = "credential-secrets"
	// transitKeyPrefix namespaces this service's Transit keys. One key per
	// category (see transitKeyName) keeps the blast radius of any single
	// key's compromise scoped to one credential category, not all of them.
	transitKeyPrefix = "credential-broker"
)

// transitKeyName derives the Transit key name for a category. Every
// WriteCredential/ResolveCredential/RotateCredential call for a given
// category uses the same key, so decrypt always targets the key encrypt
// used.
func transitKeyName(category string) string {
	return fmt.Sprintf("%s-%s", transitKeyPrefix, category)
}

// vaultPathFor derives a new credential's KV v2 path — namespaced by tenant
// so a KV v2 policy could, in principle, scope access per tenant prefix in
// the future (not implemented by this scaffold's fixed policy names above).
func vaultPathFor(tenantID, credentialID string) string {
	return fmt.Sprintf("credential/%s/%s", tenantID, credentialID)
}
