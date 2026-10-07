package domain

import (
	"reflect"
	"testing"
)

func TestSanitizeAgentFeatures(t *testing.T) {
	in := []string{
		"valid_feature",
		"",
		"a_very_long_feature_name_that_exceeds_the_limit_of_64_characters_which_is_bad",
		"duplicate",
		"duplicate",
		"non_ascii_\x7f",
		"non_ascii_chào",
		"another_valid",
	}

	// fill up to exceed 64
	for i := 0; i < 70; i++ {
		in = append(in, "feature_"+string(rune('A'+i)))
	}

	got := SanitizeAgentFeatures(in)

	if len(got) != 64 {
		t.Errorf("expected 64 features, got %d", len(got))
	}

	expectedPrefix := []string{"valid_feature", "duplicate", "another_valid"}
	if !reflect.DeepEqual(got[:3], expectedPrefix) {
		t.Errorf("expected prefix %v, got %v", expectedPrefix, got[:3])
	}
}
