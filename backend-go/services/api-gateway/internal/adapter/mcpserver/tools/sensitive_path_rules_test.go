package tools

import "testing"

func TestCleanWorktreePath_RejectsTraversalAndTricks(t *testing.T) {
	bad := []string{
		"", "..", "../etc/passwd", "a/../b", "a/..", "./a", "a/./b", "/etc/passwd", "a//b", "a/",
		`a\b`, `..\x`, `C:\Windows`, "c:/x", "a\x00b", "a\nb", "a\u202eb", "a\u200bb",
		"%2e%2e/x", "a%2fb", "a%5cb", "a%252e", "%25",
		"\uff0e\uff0e/x",          // full-width dots fold to ".." under NFKC
		"a/\uff0e\uff0e/b",        // same, nested
		"\u2025/x",                // two-dot leader
		"a\uff0fb\uff0f..\uff0fc", // full-width solidus
		"a\uff3cb",                // full-width reverse solidus
		"dir./x", "x ", "x.",
	}
	for _, p := range bad {
		if _, err := CleanWorktreePath(p); err == nil {
			t.Errorf("CleanWorktreePath(%q) must be rejected", p)
		}
	}
	good := []string{"a", "src/main.go", "docs/readme.md", ".github/workflows/ci.yml", "a b/c.txt", "ünï/çode.txt", "日本語/ファイル.txt"}
	for _, p := range good {
		if got, err := CleanWorktreePath(p); err != nil || got != p {
			t.Errorf("CleanWorktreePath(%q) = %q, %v; want unchanged", p, got, err)
		}
	}
}

func TestIsSensitivePath(t *testing.T) {
	sensitive := []string{
		".env", ".ENV", ".env.local", ".env.production", "app/.env", "deploy/server.PEM", "keys/id_rsa", "Id_Rsa", "id_ed25519.pub",
		"cert.p12", "a/b/store.keystore", ".npmrc", ".netrc", ".pypirc", ".git-credentials", ".git/config", ".GIT/config", ".git/hooks/x",
		"sub/.git/config", ".aws/credentials", "x/.ssh/id_rsa", ".kube/config", ".docker/config.json", "infra/terraform.tfstate",
		"terraform.tfstate.backup", "secrets.yaml", "config/secret.json", "db.secret", "credentials.json",
		"\uff0eenv", // full-width dot folds to ".env"
	}
	for _, p := range sensitive {
		if !IsSensitivePath(p, nil) {
			t.Errorf("IsSensitivePath(%q) = false, want true", p)
		}
	}
	fine := []string{"README.md", "src/env.go", ".env.example", ".env.sample", ".env.template", "keys.md", "git/config", "docs/secrets-policy.md", "main.go"}
	for _, p := range fine {
		if IsSensitivePath(p, nil) {
			t.Errorf("IsSensitivePath(%q) = true, want false", p)
		}
	}
	if !IsSensitivePath("conf/internal.cfg", []string{"conf/*.cfg"}) || !IsSensitivePath("a/Vault.TXT", []string{"vault.txt"}) {
		t.Error("extra globs must widen the deny set")
	}
	if IsSensitivePath(".env", []string{"*.nothing"}) != true {
		t.Error("extras must never narrow")
	}
}

func TestContainsPrivateKey(t *testing.T) {
	if !ContainsPrivateKey("x\n-----BEGIN OPENSSH PRIVATE KEY-----\nabc") || ContainsPrivateKey("-----BEGIN PUBLIC KEY-----") {
		t.Fatal("private key detection")
	}
}
