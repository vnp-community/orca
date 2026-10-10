package config

import "testing"

func TestLoad_AllowLocalRepos(t *testing.T) {
	cases := []struct {
		env     string
		set     bool
		want    bool
		wantErr bool
	}{
		{set: false, want: false},
		{env: "", set: true, want: false},
		{env: "true", set: true, want: true},
		{env: "0", set: true, want: false},
		{env: "maybe", set: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			if tc.set {
				t.Setenv("ORCA_ALLOW_LOCAL_REPOS", tc.env)
			}
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && cfg.AllowLocalRepos != tc.want {
				t.Errorf("AllowLocalRepos=%v want %v", cfg.AllowLocalRepos, tc.want)
			}
		})
	}
}
