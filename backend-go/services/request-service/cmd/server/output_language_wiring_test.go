package main

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/output_language"
)

func TestParseOutputLanguage(t *testing.T) {
	for _, c := range []struct {
		raw     string
		want    output_language.Language
		wantErr bool
	}{
		{"", output_language.Vietnamese, false},
		{"   ", output_language.Vietnamese, false},
		{"vi", output_language.Vietnamese, false},
		{"VI ", output_language.Vietnamese, false},
		{"en", output_language.English, false},
		{" En", output_language.English, false},
		{"fr", output_language.Language{}, true},
		{"vi-VN", output_language.Language{}, true},
		{"vietnamese", output_language.Language{}, true},
	} {
		got, err := parseOutputLanguage(c.raw)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseOutputLanguage(%q) = %+v, %v; want %+v (error %v)", c.raw, got, err, c.want, c.wantErr)
		}
		if err != nil {
			want := `REQUEST_DEFAULT_OUTPUT_LANGUAGE "` + c.raw + `" is not supported; use one of: vi, en`
			if err.Error() != want {
				t.Errorf("parseOutputLanguage(%q) error %q, want %q", c.raw, err, want)
			}
		}
	}
}

// The language is checked first: a bad value must surface before the DSN is looked at, let alone a store opened.
func TestRun_UnsupportedOutputLanguageStopsBeforeAnythingIsOpened(t *testing.T) {
	cfg := testConfig(t, "sqlite://x")
	cfg.OutputLanguage = "fr"
	err := run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), `REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en`) {
		t.Fatalf("want the output language error, got %v", err)
	}
	if strings.Contains(err.Error(), "unrecognized DSN scheme") {
		t.Fatalf("the DSN was looked at before the language: %v", err)
	}
}

func TestRun_SupportedOutputLanguagesGetPastTheLanguageCheck(t *testing.T) {
	for _, raw := range []string{"", "vi", "EN "} {
		cfg := testConfig(t, "sqlite://x")
		cfg.OutputLanguage = raw
		err := run(context.Background(), cfg)
		if err == nil || !strings.Contains(err.Error(), "unrecognized DSN scheme") {
			t.Errorf("REQUEST_DEFAULT_OUTPUT_LANGUAGE=%q: want the run to reach the DSN check, got %v", raw, err)
		}
	}
}
