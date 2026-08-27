package secrets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/common/secrets"
)

// fakeVault stands in for a real Vault server's Transit engine — just
// enough of the wire protocol (GET/POST transit/keys/:name,
// POST transit/sign/:name) for common/secrets' Transit methods to exercise
// against, since no real Vault instance is available in this sandbox (same
// honest gap this codebase's other passes have flagged rather than
// silently presenting a fake as equivalent to live coverage).
type fakeVault struct {
	keyExists   atomic.Bool
	createCalls atomic.Int32
}

func newFakeVaultServer(t *testing.T, fv *fakeVault) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/transit/keys/jwt-signing", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if !fv.keyExists.Load() {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"key not found"}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"type":           "rsa-2048",
					"latest_version": 2,
					"keys": map[string]any{
						"1": map[string]any{"public_key": testRSAPublicKeyPEM1},
						"2": map[string]any{"public_key": testRSAPublicKeyPEM2},
					},
				},
			})
		case http.MethodPost, http.MethodPut:
			fv.createCalls.Add(1)
			fv.keyExists.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/transit/sign/jwt-signing", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"signature":   "vault:v1:c2lnbmF0dXJlLWJ5dGVz", // base64("signature-bytes")
				"key_version": 2,
			},
		})
	})
	mux.HandleFunc("/v1/ssh/sign/dev-server-role", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"signed_key": testSignedSSHCert,
			},
		})
	})
	mux.HandleFunc("/v1/ssh/sign/missing-role", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []string{"unknown role: missing-role"},
		})
	})
	mux.HandleFunc("/v1/ssh/sign/malformed-role", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not valid json"))
	})
	return httptest.NewServer(mux)
}

func TestTransitEnsureKey_CreatesOnlyWhenMissing(t *testing.T) {
	fv := &fakeVault{}
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()

	if err := client.TransitEnsureKey(ctx, "jwt-signing", "rsa-2048"); err != nil {
		t.Fatalf("first TransitEnsureKey: %v", err)
	}
	if got := fv.createCalls.Load(); got != 1 {
		t.Fatalf("expected 1 create call after first ensure, got %d", got)
	}

	if err := client.TransitEnsureKey(ctx, "jwt-signing", "rsa-2048"); err != nil {
		t.Fatalf("second TransitEnsureKey: %v", err)
	}
	if got := fv.createCalls.Load(); got != 1 {
		t.Fatalf("expected create to stay idempotent (still 1 call), got %d", got)
	}
}

func TestTransitPublicKeyVersions(t *testing.T) {
	fv := &fakeVault{}
	fv.keyExists.Store(true)
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	versions, latest, err := client.TransitPublicKeyVersions(context.Background(), "jwt-signing")
	if err != nil {
		t.Fatalf("TransitPublicKeyVersions: %v", err)
	}
	if latest != 2 {
		t.Fatalf("expected latest version 2, got %d", latest)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[1] != testRSAPublicKeyPEM1 || versions[2] != testRSAPublicKeyPEM2 {
		t.Fatalf("unexpected version contents: %+v", versions)
	}

	pem, version, err := client.TransitPublicKey(context.Background(), "jwt-signing")
	if err != nil {
		t.Fatalf("TransitPublicKey: %v", err)
	}
	if version != 2 || pem != testRSAPublicKeyPEM2 {
		t.Fatalf("TransitPublicKey should return the latest version, got version=%d", version)
	}
}

func TestTransitSign_ParsesSignatureField(t *testing.T) {
	fv := &fakeVault{}
	fv.keyExists.Store(true)
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	sig, err := client.TransitSign(context.Background(), "jwt-signing", []byte("payload"))
	if err != nil {
		t.Fatalf("TransitSign: %v", err)
	}
	if sig != "vault:v1:c2lnbmF0dXJlLWJ5dGVz" {
		t.Fatalf("unexpected signature wire value: %q", sig)
	}
}

func TestSSHSignPublicKey_Success(t *testing.T) {
	fv := &fakeVault{}
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	signed, err := client.SSHSignPublicKey(context.Background(), "dev-server-role", "ssh-ed25519 AAAAtest test@host")
	if err != nil {
		t.Fatalf("SSHSignPublicKey: %v", err)
	}
	if signed != testSignedSSHCert {
		t.Fatalf("unexpected signed cert: got %q, want %q", signed, testSignedSSHCert)
	}
}

func TestSSHSignPublicKey_VaultErrorResponse(t *testing.T) {
	fv := &fakeVault{}
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	signed, err := client.SSHSignPublicKey(context.Background(), "missing-role", "ssh-ed25519 AAAAtest test@host")
	if err == nil {
		t.Fatalf("expected an error for an unknown Vault SSH role, got signed=%q", signed)
	}
	if signed != "" {
		t.Fatalf("expected empty signed cert on error, got %q", signed)
	}
}

func TestSSHSignPublicKey_MalformedResponse(t *testing.T) {
	fv := &fakeVault{}
	server := newFakeVaultServer(t, fv)
	defer server.Close()
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	signed, err := client.SSHSignPublicKey(context.Background(), "malformed-role", "ssh-ed25519 AAAAtest test@host")
	if err == nil {
		t.Fatalf("expected an error for a malformed Vault response, got signed=%q", signed)
	}
	if signed != "" {
		t.Fatalf("expected empty signed cert on error, got %q", signed)
	}
}

// testSignedSSHCert is an opaque string standing in for a real
// OpenSSH-format signed certificate — SSHSignPublicKey treats this field as
// opaque (it never parses cert material itself; sshconn's Connector does
// that, tested separately against a real generated certificate), so this
// only needs to be a distinct, non-empty string.
const testSignedSSHCert = "ssh-ed25519-cert-v01@openssh.com AAAAfakesignedcertdata test-signed-cert"

// Two distinct opaque PEM-shaped strings standing in for two Transit key
// versions' public halves. common/secrets treats this field as opaque (it
// never parses key material — that's jwtauth's job, tested separately with
// a real generated RSA key), so these only need to be distinct, not valid
// PKIX-encoded keys.
const testRSAPublicKeyPEM1 = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwzq5UzOu+Sq5r5Hn4/pn
JGYyC4gCPVDpJs8T1jP44e3xhh8meDDwPGgN/GgHnxXt5N4v/dl64BzMlsFRp/pJ
gCq83sV0IsxucIw8pOAxxsvcAcLW56hxlvkQ6HxvS26cLKmuI/pumcE94w1BQnQK
Uz13AbAV5kFyLPo6zsEuMnb5foljyqORf83WrPPtKQx36xnhc02+SPTGkFa0BQ/z
6bx7nMWG1qxNXWLd52lWyC2TibC/CnKYqDskDDZ6t20vN7oPRRLTz9d1U55HqPfp
oOxieVR22Y+tI06CkzlodO0X+Db+7DXbTQ6SGz51F3XSXPo6Zz2AKe0F1P+dqp/N
9wIDAQAB
-----END PUBLIC KEY-----`

const testRSAPublicKeyPEM2 = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxU4nZ4UO0h2fRT6zA1L4
kCMdmPqvxpvGxK1F9SGnceoBWPKAmpUxULoscNK0ZK9pT4Nz3M27KzfBLxOe2GwK
h5Eb4uedn79xZzKvyoBHi/mQ7EhBSy3qksC2u8OGvHz2SzHmv7NM4/OG55O87OMh
5tRDe7SdQfxb56gh8Qffi27iYUWQGyN/HAxQb0hRLWKQ4qEEeVJgVKDCVIiv2FpA
zdHqCK1eGuLmb+xCU6uxi8f/uEXaK4o2FZ0BkALqUb2X2i3+RPQBv3Wcm42sZAtQ
sMSGH0S1lLoQxKuHYVwqxpvbhMwSw+O44wLTNyCf1XyChZ3Xnh6bK2hZKcYh5xJl
CwIDAQAB
-----END PUBLIC KEY-----`
