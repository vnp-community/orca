package domain

import "testing"

func TestDefaultTenantSettings_OnlyEnabledIsConfigurable(t *testing.T) {
	on := DefaultTenantSettings("t", true, 90)
	off := DefaultTenantSettings("t", false, 90)
	if !on.Enabled || off.Enabled {
		t.Fatal("enabled must follow the flag")
	}
	on.Enabled = false
	if on != off {
		t.Fatalf("flag changed more than `enabled`: %+v vs %+v", on, off)
	}
	if off.KillSwitch.Active || off.DCREnabled {
		t.Fatal("safety defaults must stay off")
	}
}

func TestTenantSettingsValidate(t *testing.T) {
	ok := DefaultTenantSettings("t", true, 90)
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TenantSettings){
		"no tenant":   func(s *TenantSettings) { s.TenantID = "" },
		"days zero":   func(s *TenantSettings) { s.MaxTokenDays = 0 },
		"days 91":     func(s *TenantSettings) { s.MaxTokenDays = 91 },
		"ttl low":     func(s *TenantSettings) { s.ApprovalTTLSeconds = 29 },
		"ttl too big": func(s *TenantSettings) { s.ApprovalTTLSeconds = 86401 },
	}
	for name, mutate := range cases {
		s := ok
		mutate(&s)
		if s.Validate() == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestScopeCatalog_RisksAreContractValues(t *testing.T) {
	valid := map[Risk]bool{RiskRead: true, RiskWriteReversible: true, RiskExec: true, RiskDestructive: true, RiskAdmin: true}
	got := ScopeCatalog()
	if len(got) != 4 {
		t.Fatalf("want 4 scopes, got %d", len(got))
	}
	for _, s := range got {
		if !valid[s.Risk] || s.ID == "" || s.Label == "" {
			t.Errorf("bad scope %+v", s)
		}
	}
}
