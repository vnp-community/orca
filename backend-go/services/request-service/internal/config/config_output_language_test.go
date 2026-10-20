package config

import "testing"

func TestLoad_OutputLanguageIsTheTrimmedRawValue(t *testing.T) {
	for _, c := range []struct{ env, want string }{{"", ""}, {"en", "en"}, {" VI ", "VI"}, {"fr", "fr"}} {
		t.Setenv("REQUEST_DEFAULT_OUTPUT_LANGUAGE", c.env)
		if got := Load().OutputLanguage; got != c.want {
			t.Errorf("REQUEST_DEFAULT_OUTPUT_LANGUAGE=%q: OutputLanguage = %q, want %q", c.env, got, c.want)
		}
	}
}
