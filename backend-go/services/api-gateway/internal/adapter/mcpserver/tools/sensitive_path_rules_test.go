package tools

import "testing"



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
