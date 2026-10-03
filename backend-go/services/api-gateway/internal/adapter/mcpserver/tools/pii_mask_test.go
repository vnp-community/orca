package tools

import (
	"strings"
	"testing"
)

func TestMaskPIIString(t *testing.T) {
	for in, want := range map[string]string{
		"mail alice.smith@example.com now":     "mail a***@example.com now",
		"call +84 912 345 678":                 "call +** *** *** *78",
		"tel 0912 345 678":                     "tel **** *** *78",
		"ssn 123-45-6789":                      "ssn ***-**-**89",
		"id 012345678901":                      "id **********01",
		"2026-10-03 12:30:00":                  "2026-10-03 12:30:00",
		"550e8400-e29b-41d4-a716-446655440000": "550e8400-e29b-41d4-a716-446655440000",
		"commit abc1234 on main":               "commit abc1234 on main",
	} {
		if got := maskPIIString(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestPIIMaskModes(t *testing.T) {
	dir := &ToolSpec{PII: true}
	plain := &ToolSpec{}
	if !piiApplies("directory", dir) || piiApplies("directory", plain) || !piiApplies("all", plain) || piiApplies("off", dir) {
		t.Error("mode matrix wrong")
	}
	v := map[string]any{"items": []any{map[string]any{"email": "bob@corp.io", "phone": "0901234567", "name": "Bob"}}}
	out := maskPII(v).(map[string]any)["items"].([]any)[0].(map[string]any)
	if out["email"] != "b***@corp.io" || !strings.HasSuffix(out["phone"].(string), "67") || strings.Contains(out["phone"].(string), "0901") || out["name"] != "Bob" {
		t.Errorf("%v", out)
	}
}
