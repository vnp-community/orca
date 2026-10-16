package domain

import (
	"testing"
)

func TestEffectiveEngine_Table(t *testing.T) {
	profiles := []OpenSpecProfile{OpenSpecProfileFull, OpenSpecProfileLight, OpenSpecProfileNone}
	engineNative := EngineNative
	engineOpenSpec := EngineOpenSpec
	pinnedOptions := []*EngineName{nil, &engineNative, &engineOpenSpec}

	settingsOptions := []*ProjectEngineSettings{
		nil,
		{Engine: EngineNative},
		{Engine: EngineOpenSpec},
	}

	count := 0
	for _, prof := range profiles {
		for _, pinned := range pinnedOptions {
			for _, settings := range settingsOptions {
				count++
				res := EffectiveEngine(pinned, settings, prof)
				if prof == OpenSpecProfileNone {
					if res != EngineNative {
						t.Errorf("expected native for none profile, got %s", res)
					}
				} else {
					if pinned != nil {
						if res != *pinned {
							t.Errorf("expected pinned %s, got %s", *pinned, res)
						}
					} else if settings != nil && settings.Engine != "" {
						if res != settings.Engine {
							t.Errorf("expected settings %s, got %s", settings.Engine, res)
						}
					} else {
						if res != EngineNative {
							t.Errorf("expected fallback native, got %s", res)
						}
					}
				}
			}
		}
	}
	if count != 27 {
		t.Errorf("expected 27 tests, ran %d", count)
	}
}

func TestEngineDefaultsFromEnv(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		name  string
		env   map[string]string
		want  EngineDefaults
		warns int
	}{
		{"unset keeps native and off", nil, EngineDefaults{Engine: EngineNative, ExecutionMode: ExecutionStepByStep}, 0},
		{"both on", map[string]string{"REQUEST_DEFAULT_ENGINE": " OpenSpec ", "REQUEST_DEFAULT_SOLUTION_AGENT": "true"}, EngineDefaults{Engine: EngineOpenSpec, SolutionAgent: true, ExecutionMode: ExecutionStepByStep}, 0},
		{"continuous mode", map[string]string{"REQUEST_DEFAULT_EXECUTION_MODE": " Continuous "}, EngineDefaults{Engine: EngineNative, ExecutionMode: ExecutionContinuous}, 0},
		{"unreadable values keep the safe default and warn", map[string]string{"REQUEST_DEFAULT_ENGINE": "fast", "REQUEST_DEFAULT_SOLUTION_AGENT": "maybe", "REQUEST_DEFAULT_EXECUTION_MODE": "turbo"}, EngineDefaults{Engine: EngineNative, ExecutionMode: ExecutionStepByStep}, 3},
	}
	for _, c := range cases {
		got, warns := EngineDefaultsFromEnv(get(c.env))
		if got != c.want || len(warns) != c.warns {
			t.Errorf("%s: got %+v %v", c.name, got, warns)
		}
	}
}
