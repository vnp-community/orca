# Vault policy for Orca's backend-go services against the SHARED Vault
# (vnp-domain/vault/, 172.20.2.21) — NOT yet applied anywhere. See
# VAULT-SHARED-MIGRATION.md in this directory for the full migration plan
# and why this hasn't been applied live.
#
# Scoped to exactly what backend-go/common/secrets (transit + credential-secrets
# KV v2) and auth-service's own direct Transit JWT-signing use touch — see
# common/secrets/vault.go's doc comment for why those two callers are the only
# ones with direct Vault access (every other service only reads dynamic DB
# credentials via a Vault Agent sidecar file, not via a token/policy at all).
#
# Requires these two engines to be enabled first (not done by this policy):
#   vault secrets enable transit
#   vault secrets enable -path=credential-secrets kv-v2

# credential-broker-service: encrypt/decrypt tenant secret material.
# auth-service: sign/verify its own JWT signing key (a service identity key,
# not tenant secret material — see vault.go's doc comment on why this is a
# documented exception, not a violation of "only credential-broker-service
# touches tenant secrets directly").
# Why "create" alongside "update" on transit/encrypt/*, not just decrypt/sign/
# verify: encrypting against a not-yet-existing key name auto-vivifies it —
# live-verified (2026-09-07): "update" alone 403s on first use against a new
# key name; Vault's ACL treats that auto-create as a "create" operation on
# the encrypt path itself, separate from transit/keys/* below. Decrypt/sign/
# verify never auto-create (they require the key to already exist), so those
# stay "update"-only.
path "transit/encrypt/*" {
  capabilities = ["create", "update"]
}
path "transit/decrypt/*" {
  capabilities = ["update"]
}
path "transit/sign/*" {
  capabilities = ["update"]
}
path "transit/verify/*" {
  capabilities = ["update"]
}
# Explicit key management (rotate/config) — not currently called by this
# codebase (no separate "create key" usecase), kept narrow: read for
# introspection, update for the rare config change, no delete/list.
path "transit/keys/*" {
  capabilities = ["read", "update"]
}

# notification-service's per-tenant Web Push (VAPID) signing key. Unlike the
# encrypt keys above it can NOT auto-vivify: signing needs an ecdsa-p256 key,
# whereas a Transit key created implicitly by an encrypt call is aes256-gcm96.
# scripts/provision-vapid-key.sh creates "vapid-signing-<tenant_id>" explicitly,
# which needs "create" on exactly this prefix and nothing broader. Since the
# broker's EnsureVapidSigningKey RPC, credential-broker-service creates the key
# itself on a tenant's first Web Push use (needs create+read here); the script
# is only an optional pre-warm. This policy step stays required. (The
# credential-broker "mcp_external_secret" key needs no entry: it is aes256-gcm96
# and auto-vivifies under transit/encrypt/* above.) Apply with:
#   vault policy write <policy attached to the orca token> deploy/dev/orca-policy.hcl
# (find the name with `vault token lookup` -> policies; the token itself is
# called orca-backend-go, but VAULT-SHARED-MIGRATION.md never records the policy's name)
path "transit/keys/vapid-signing-*" {
  capabilities = ["create", "read", "update"]
}

# credential-broker-service: KV v2 read/write for ciphertext storage
# (WriteCredential/ResolveCredential), and destroy-metadata for permanent
# revocation (RevokeSecret → KVDestroyMetadata, see secret_store.go's doc
# comment on why this is a hard delete, not an overwrite).
path "credential-secrets/data/*" {
  capabilities = ["create", "update", "read"]
}
path "credential-secrets/metadata/*" {
  capabilities = ["read", "delete"]
}

# Health check (SecretStore.Ping → Sys().Health()) — a Vault system endpoint,
# not covered by the two mounts above.
path "sys/health" {
  capabilities = ["read", "sudo"]
}
