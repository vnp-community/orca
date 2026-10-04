package config

import "testing"

func TestWSOriginModeValidation(t *testing.T) {
	for _, tc := range []struct {
		env     string
		want    string
		wantErr bool
	}{
		{"", "enforce", false},
		{"enforce", "enforce", false},
		{" REPORT ", "report", false},
		{"audit", "", true},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("WS_ORIGIN_MODE", tc.env)
			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.WSOriginMode != tc.want {
				t.Fatalf("mode=%q want %q", cfg.WSOriginMode, tc.want)
			}
		})
	}
}
