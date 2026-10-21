package domain

import "testing"

func TestDrawEngineAndSkillPinDefaults(t *testing.T) {
	cfg, warn := OutcomeReviewConfigFromEnv(func(string) string { return "" })
	if len(warn) != 0 || cfg.DrawEngineOrDefault() != DrawEngineAuto || cfg.EffectiveDrawSkillSHA() != VendoredDiagramDesignSHA {
		t.Fatalf("defaults: engine %q pin %q warnings %v", cfg.DrawEngineOrDefault(), cfg.EffectiveDrawSkillSHA(), warn)
	}
	env := map[string]string{"REQUEST_REVIEW_DRAW_ENGINE": "builtin", "REQUEST_REVIEW_DRAW_SKILL_SHA": "ANY"}
	cfg, _ = OutcomeReviewConfigFromEnv(func(k string) string { return env[k] })
	if cfg.DrawEngineOrDefault() != DrawEngineBuiltin || cfg.EffectiveDrawSkillSHA() != "" {
		t.Fatalf("overrides: engine %q pin %q", cfg.DrawEngineOrDefault(), cfg.EffectiveDrawSkillSHA())
	}
	env = map[string]string{"REQUEST_REVIEW_DRAW_ENGINE": "nonsense", "REQUEST_REVIEW_DRAW_SKILL_SHA": "zz"}
	cfg, warn = OutcomeReviewConfigFromEnv(func(k string) string { return env[k] })
	if cfg.DrawEngineOrDefault() != DrawEngineAuto || cfg.EffectiveDrawSkillSHA() != VendoredDiagramDesignSHA || len(warn) != 2 {
		t.Fatalf("bad values must fall back with a warning each: %q %q %v", cfg.DrawEngineOrDefault(), cfg.EffectiveDrawSkillSHA(), warn)
	}
	pin := "0123456789abcdef"
	env = map[string]string{"REQUEST_REVIEW_DRAW_SKILL_SHA": pin}
	if cfg, _ = OutcomeReviewConfigFromEnv(func(k string) string { return env[k] }); cfg.EffectiveDrawSkillSHA() != pin {
		t.Fatalf("a deployment pin: %q", cfg.EffectiveDrawSkillSHA())
	}
}
