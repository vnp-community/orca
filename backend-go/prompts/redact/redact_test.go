package redact

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const sample = "contact bob@example.com token ghp_abcdefghijklmnopqrstuvwxyz0123456789 Bearer abc12345def"

type fixedSettings struct {
	redact bool
	err    error
}

func (f fixedSettings) RedactPII(context.Context) (bool, error) { return f.redact, f.err }

type maskEmails struct{}

func (maskEmails) Mask(s string) string { return strings.ReplaceAll(s, "bob@example.com", "[email]") }

func TestRedactor_SecretsAlwaysPIIOnRequest(t *testing.T) {
	off := NewRedactor(fixedSettings{redact: false}, maskEmails{}).Apply(context.Background(), sample)
	if strings.Contains(off, "ghp_abcdef") || strings.Contains(off, "abc12345def") || !strings.Contains(off, "bob@example.com") {
		t.Errorf("secrets always masked, PII only on request: %q", off)
	}
	on := NewRedactor(fixedSettings{redact: true}, maskEmails{}).Apply(context.Background(), sample)
	if strings.Contains(on, "bob@example.com") {
		t.Errorf("PII must be masked when the tenant asks: %q", on)
	}
}

func TestRedactor_SettingsErrorFailsClosed(t *testing.T) {
	failed := NewRedactor(fixedSettings{err: errors.New("db")}, maskEmails{}).Apply(context.Background(), sample)
	if strings.Contains(failed, "bob@example.com") {
		t.Errorf("a settings error must mask PII: %q", failed)
	}
}

func TestRedactor_NoMaskerOrSettings(t *testing.T) {
	if got := NewRedactor(nil, nil).Apply(context.Background(), "plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
	withoutSettings := NewRedactor(nil, maskEmails{}).Apply(context.Background(), sample)
	if strings.Contains(withoutSettings, "bob@example.com") {
		t.Errorf("no settings reader must default to masking PII: %q", withoutSettings)
	}
}

func TestSecrets_MasksCredentials(t *testing.T) {
	if got := Secrets("key AKIAIOSFODNN7EXAMPLE"); strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("secret survived: %q", got)
	}
}
