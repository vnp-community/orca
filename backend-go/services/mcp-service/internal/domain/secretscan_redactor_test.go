package domain

import (
	"strings"
	"testing"
)

func TestSecretscanRedactor_MasksSharedPatterns(t *testing.T) {
	var r Redactor = SecretscanRedactor{}
	in := "token ghp_abcdefghijklmnopqrstuvwxyz0123456789 and key sk-ant-abcdefghijklmnopqrstuvwx"
	out, changed := r.Redact(in)
	if !changed || strings.Contains(out, "ghp_abcdef") || strings.Contains(out, "sk-ant-abcdef") {
		t.Fatalf("not masked: %q", out)
	}
	if out2, changed2 := r.Redact("plain text"); changed2 || out2 != "plain text" {
		t.Fatalf("clean text changed: %q", out2)
	}
	// Anything the legacy redactor masks for these inputs is masked here too (never exposes more).
	legacy, _ := SecretRedactor{}.Redact(in)
	if strings.Contains(out, "ghp_") != strings.Contains(legacy, "ghp_") {
		t.Fatal("coverage differs on a github token")
	}
}
