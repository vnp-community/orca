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
