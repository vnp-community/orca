package output_language

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, c := range []struct {
		raw     string
		want    Language
		wantErr bool
	}{
		{"vi", Vietnamese, false},
		{"VI ", Vietnamese, false},
		{"  En", English, false},
		{"en", English, false},
		{"fr", Language{}, true},
		{"", Language{}, true},
		{"   ", Language{}, true},
		{"vi-VN", Language{}, true},
		{"vietnamese", Language{}, true},
	} {
		got, err := Parse(c.raw)
		if (err != nil) != c.wantErr {
			t.Errorf("Parse(%q) error = %v, wantErr %v", c.raw, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.raw, got, c.want)
		}
		if err != nil && !strings.Contains(err.Error(), "use one of: vi, en") {
			t.Errorf("Parse(%q) error %q does not list the valid codes", c.raw, err)
		}
	}
}

func TestSupported_FixedOrderAndZeroValueIsNotSupported(t *testing.T) {
	got := Supported()
	if len(got) != 2 || got[0] != Vietnamese || got[1] != English {
		t.Fatalf("Supported() = %+v", got)
	}
	for _, l := range got {
		if l.Code == "" || l.Name == "" {
			t.Errorf("language %+v lacks a code or a name", l)
		}
	}
	if (Language{}).Code != "" {
		t.Fatal("the zero Language must have no code: templates treat it as no instruction")
	}
}
