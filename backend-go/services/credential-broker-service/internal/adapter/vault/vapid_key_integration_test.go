//go:build integration

// Runs the real adapter against a throwaway Vault dev container; run with
// `go test -tags=integration ./internal/adapter/vault/...`.
package vault_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/stablyai/orca-go/common/secrets"
	"github.com/stablyai/orca-go/common/tenant"
	credentialvault "github.com/stablyai/orca-go/services/credential-broker-service/internal/adapter/vault"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

const rootToken = "orca-it-root"

// startVaultDev starts hashicorp/vault:1.17 in dev mode and returns its URL.
func startVaultDev(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("docker", "run", "-d", "--rm", "--cap-add=IPC_LOCK",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID="+rootToken,
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"-p", "127.0.0.1::8200", "hashicorp/vault:1.17").CombinedOutput()
	if err != nil {
		t.Skipf("docker unavailable: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	portOut, err := exec.Command("docker", "port", id, "8200/tcp").Output()
	if err != nil {
		t.Fatalf("docker port: %v", err)
	}
	line := strings.Split(strings.TrimSpace(string(portOut)), "\n")[0]
	addr := "http://127.0.0.1:" + line[strings.LastIndex(line, ":")+1:]

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr + "/v1/sys/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return addr
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("vault dev server did not become healthy")
	return ""
}

func apiClient(t *testing.T, addr, token string) *vaultapi.Client {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	c, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.SetToken(token)
	return c
}

func secretsClient(t *testing.T, addr, token string) *secrets.Client {
	t.Helper()
	t.Setenv("VAULT_ADDR", addr)
	t.Setenv("VAULT_TOKEN", token)
	c, err := secrets.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEnsureVapidSigningKey_AgainstRealVault(t *testing.T) {
	addr := startVaultDev(t)
	root := apiClient(t, addr, rootToken)
	ctx := context.Background()
	if _, err := root.Logical().WriteWithContext(ctx, "sys/mounts/transit", map[string]any{"type": "transit"}); err != nil {
		t.Fatalf("enabling transit: %v", err)
	}

	store := credentialvault.New(secretsClient(t, addr, rootToken))
	uc := usecase.NewEnsureVapidSigningKey(store)
	in := usecase.EnsureVapidSigningKeyInput{RequestingService: "notification-service"}
	tctx := tenant.WithTenantID(ctx, "tenant-it")

	// Concurrent first calls: one key, identical public key for everyone.
	var wg sync.WaitGroup
	results := make([]usecase.EnsureVapidSigningKeyResult, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = uc.Execute(tctx, in)
		}()
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if results[i].PublicKey != results[0].PublicKey {
			t.Fatalf("public keys differ between concurrent callers")
		}
	}
	pubB64 := results[0].PublicKey
	if len(pubB64) != 87 {
		t.Fatalf("public key length %d, want 87", len(pubB64))
	}

	// Key type is ecdsa-p256.
	secret, err := root.Logical().ReadWithContext(ctx, "transit/keys/vapid-signing-tenant-it")
	if err != nil || secret == nil {
		t.Fatalf("reading key: %v", err)
	}
	if got := secret.Data["type"]; got != "ecdsa-p256" {
		t.Fatalf("key type %v, want ecdsa-p256", got)
	}

	// Second call reuses and reports created=false with the same key.
	again, err := uc.Execute(tctx, in)
	if err != nil || again.Created || again.PublicKey != pubB64 {
		t.Fatalf("second call: %+v err=%v", again, err)
	}

	// Vault-produced signature verifies against the Go-derived key.
	raw, err := base64.RawURLEncoding.DecodeString(pubB64)
	if err != nil || raw[0] != 0x04 {
		t.Fatalf("bad public key encoding: %v", err)
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(raw[1:33]), Y: new(big.Int).SetBytes(raw[33:])}
	input := []byte("eyJ0eXAiOiJKV1QiLCJhbGciOiJFUzI1NiJ9.eyJhdWQiOiJ4In0")
	sigWire, err := usecase.NewSignVapidPayload(store).Execute(ctx, usecase.SignVapidPayloadInput{TenantID: "tenant-it", Payload: input})
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	parts := strings.SplitN(sigWire, ":", 3)
	der, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(input)
	if !ecdsa.VerifyASN1(pub, digest[:], der) {
		t.Fatal("Vault signature does not verify against the derived public key")
	}

	// A pre-existing key of another type is refused and left intact.
	if _, err := root.Logical().WriteWithContext(ctx, "transit/keys/vapid-signing-tenant-aes", map[string]any{"type": "aes256-gcm96"}); err != nil {
		t.Fatal(err)
	}
	_, err = uc.Execute(tenant.WithTenantID(ctx, "tenant-aes"), in)
	if err == nil || !strings.Contains(err.Error(), usecase.CodeVapidKeyTypeMismatch) {
		t.Fatalf("want type mismatch, got %v", err)
	}
	still, _ := root.Logical().ReadWithContext(ctx, "transit/keys/vapid-signing-tenant-aes")
	if still == nil || still.Data["type"] != "aes256-gcm96" {
		t.Fatal("wrong-type key was modified or deleted")
	}

	// A token without the policy yields the typed forbidden error.
	if _, err := root.Logical().WriteWithContext(ctx, "sys/policies/acl/readonly-transit", map[string]any{
		"policy": `path "transit/keys/*" { capabilities = ["read"] }`,
	}); err != nil {
		t.Fatal(err)
	}
	tok, err := root.Auth().Token().CreateWithContext(ctx, &vaultapi.TokenCreateRequest{Policies: []string{"readonly-transit"}})
	if err != nil {
		t.Fatal(err)
	}
	weak := credentialvault.New(secretsClient(t, addr, tok.Auth.ClientToken))
	_, err = usecase.NewEnsureVapidSigningKey(weak).Execute(tenant.WithTenantID(ctx, "tenant-new"), in)
	if err == nil || !strings.Contains(err.Error(), usecase.CodeVaultForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if k, _ := root.Logical().ReadWithContext(ctx, "transit/keys/vapid-signing-tenant-new"); k != nil {
		t.Fatal("key must not exist after forbidden create")
	}

	// Unreachable Vault -> unavailable.
	dead := credentialvault.New(secretsClient(t, "http://127.0.0.1:1", rootToken))
	_, err = usecase.NewEnsureVapidSigningKey(dead).Execute(tctx, in)
	if err == nil || !strings.Contains(err.Error(), usecase.CodeVaultUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}
