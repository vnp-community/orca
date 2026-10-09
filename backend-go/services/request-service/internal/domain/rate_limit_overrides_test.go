package domain

import "testing"

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLimitsFromEnv_DefaultsWhenUnset(t *testing.T) {
	got, warns := LimitsFromEnv(envOf(nil))
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings %v", warns)
	}
	for class, want := range DefaultLimits {
		if got[class] != want {
			t.Errorf("%s = %+v, want default %+v", class, got[class], want)
		}
	}
}

func TestLimitsFromEnv_OverridesOneClassOnly(t *testing.T) {
	got, warns := LimitsFromEnv(envOf(map[string]string{"REQUEST_RATE_WRITE_PER_SECOND": "50", "REQUEST_RATE_WRITE_BURST": "100"}))
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings %v", warns)
	}
	if got[RateWrite] != (Limit{PerSecond: 50, Burst: 100}) {
		t.Errorf("write = %+v", got[RateWrite])
	}
	if got[RateRead] != DefaultLimits[RateRead] {
		t.Errorf("read changed to %+v", got[RateRead])
	}
}

func TestLimitsFromEnv_InvalidValuesKeepDefaultAndWarn(t *testing.T) {
	got, warns := LimitsFromEnv(envOf(map[string]string{
		"REQUEST_RATE_AI_PER_SECOND": "fast", "REQUEST_RATE_AI_BURST": "0", "REQUEST_RATE_READ_PER_SECOND": "-3",
	}))
	if len(warns) != 3 {
		t.Fatalf("want 3 warnings, got %v", warns)
	}
	if got[RateAI] != DefaultLimits[RateAI] || got[RateRead] != DefaultLimits[RateRead] {
		t.Errorf("invalid input must keep defaults: ai=%+v read=%+v", got[RateAI], got[RateRead])
	}
}
