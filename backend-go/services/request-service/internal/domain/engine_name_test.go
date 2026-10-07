package domain

import (
	"testing"
)

func TestParseEngineName_ValidAndInvalid(t *testing.T) {
	for _, en := range AllEngineNames() {
		parsed, err := ParseEngineName(string(en))
		if err != nil {
			t.Errorf("expected no error for %s, got %v", en, err)
		}
		if parsed != en {
			t.Errorf("expected %s, got %s", en, parsed)
		}
	}

	_, err := ParseEngineName("foo")
	if err != ErrEngineInvalid {
		t.Errorf("expected ErrEngineInvalid, got %v", err)
	}

	_, err = ParseEngineName("")
	if err != ErrEngineInvalid {
		t.Errorf("expected ErrEngineInvalid, got %v", err)
	}
}
