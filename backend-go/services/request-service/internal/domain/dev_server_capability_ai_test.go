package domain

import (
	"testing"
	"time"
)

func parse(t *testing.T, profile string) DevServerCapability {
	t.Helper()
	c, err := ParseCapabilityProfile("agent", false, true, "2.2.0", 2, nil, []byte(profile), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAIReadiness(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		want    AIReadiness
	}{
		{"agent reports a CLI login", `{"schemaVersion":1,"aiSources":["cli_session"]}`, AIReady},
		{"agent reports an env key", `{"schemaVersion":1,"aiSources":["env_key"]}`, AIReady},
		{"agent reports no source", `{"schemaVersion":1,"aiSources":[]}`, AINotConfigured},
		{"empty sources on a partial probe prove nothing", `{"schemaVersion":1,"partial":true,"aiSources":[]}`, AIReadinessUnknown},
		{"older agent, logged-in claude that can disable tools", `{"schemaVersion":1,"claude":{"installed":true,"auth":"logged_in","flags":{"tools":true}},"env":[]}`, AIReady},
		{"older agent, key present", `{"schemaVersion":1,"claude":{"installed":false,"auth":"unknown"},"env":[{"name":"ANTHROPIC_API_KEY","present":true}]}`, AIReady},
		{"older agent, logged out and no key", `{"schemaVersion":1,"claude":{"installed":true,"auth":"logged_out"},"env":[{"name":"ANTHROPIC_API_KEY","present":false}]}`, AINotConfigured},
		{"older agent, login not probed", `{"schemaVersion":1,"claude":{"installed":true,"auth":"unknown"},"env":[]}`, AIReadinessUnknown},
		{"no probe data at all", `{"schemaVersion":1}`, AIReadinessUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parse(t, tc.profile).AIReadiness(); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseCapabilityProfile_ReadsAISources(t *testing.T) {
	c := parse(t, `{"schemaVersion":1,"aiSources":["env_key","cli_session"]}`)
	if !c.AISourcesKnown || len(c.AISources) != 2 || c.AISources[1] != "cli_session" {
		t.Fatalf("unexpected: %+v", c)
	}
	if parse(t, `{"schemaVersion":1}`).AISourcesKnown {
		t.Fatal("missing aiSources must stay unknown")
	}
}
