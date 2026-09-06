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
path "transit/encrypt/*" {
  capabilities = ["update"]
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
# Transit key creation is implicit-on-first-use in this codebase (no separate
# "create key" usecase) — encrypt/decrypt/sign/verify against a not-yet-existing
# key name auto-vivifies it, which requires "create" alongside "update" on the
# same paths per Vault's Transit engine semantics.
path "transit/keys/*" {
  capabilities = ["create", "update"]
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
