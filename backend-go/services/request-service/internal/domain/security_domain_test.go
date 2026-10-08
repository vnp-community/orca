package domain

import (
	"strings"
	"testing"
	"time"
)

func TestMarshalAuditMetadata_ForbiddenKeysAtAnyDepth(t *testing.T) {
	for name, m := range map[string]map[string]any{
		"top":    {"title": "x"},
		"nested": {"a": map[string]any{"b": map[string]any{"body": "x"}}},
		"list":   {"items": []any{map[string]any{"Comment": "x"}}},
		"case":   {"PROMPT": "x"},
	} {
		if _, err := MarshalAuditMetadata("r1", "a1", m); err == nil {
			t.Errorf("%s: forbidden key accepted", name)
		}
	}
}

func TestMarshalAuditMetadata_LongStringAndSizeRejected(t *testing.T) {
	if _, err := MarshalAuditMetadata("r", "a", map[string]any{"note": strings.Repeat("x", 300)}); err == nil {
		t.Error("300 character string must be rejected")
	}
	big := map[string]any{}
	for i := 0; i < 200; i++ {
		big[strings.Repeat("k", 10)+string(rune('a'+i%26))+string(rune('a'+i/26))] = strings.Repeat("v", 200)
	}
	if _, err := MarshalAuditMetadata("r", "a", big); err == nil {
		t.Error("over 4096 bytes must be rejected")
	}
}

func TestMarshalAuditMetadata_AddsIDs(t *testing.T) {
	out, err := MarshalAuditMetadata("req-1", "aud-1", map[string]any{"count": 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"request_id":"req-1"`, `"audit_id":"aud-1"`, `"count":3`} {
		if !strings.Contains(out, want) {
			t.Errorf("%s missing from %s", want, out)
		}
	}
}

func TestIsDurableAudit(t *testing.T) {
	for _, a := range []string{AuditRequestExport, AuditRequestErase, AuditAIEgressSet, AuditRetentionRun} {
		if !IsDurableAudit(a, "") {
			t.Errorf("%s must be durable", a)
		}
	}
	if IsDurableAudit(AuditApprovalApprove, "solution") || !IsDurableAudit(AuditApprovalApprove, "pre_deploy") {
		t.Error("approval.approve is durable only for pre_deploy")
	}
	if IsDurableAudit(AuditRequestAccessDenied, "") {
		t.Error("access denied stays best effort")
	}
}

func TestPlanFor(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.FixedZone("x", 7*3600))
	p := PlanFor("t", DefaultRetention, now)
	if want := now.UTC().AddDate(0, 0, -730); !p.RequestCutoff.Equal(want) || p.RequestCutoff.Location() != time.UTC {
		t.Errorf("request cutoff = %v, want %v in UTC", p.RequestCutoff, want)
	}
	if want := now.UTC().AddDate(0, 0, -30); !p.TraceCutoff.Equal(want) {
		t.Errorf("trace cutoff = %v, want %v", p.TraceCutoff, want)
	}
	keep := PlanFor("t", RetentionSettings{}, now)
	if !keep.RequestCutoff.IsZero() || !keep.TraceCutoff.IsZero() || !keep.LedgerCutoff.IsZero() {
		t.Error("0 days keeps everything")
	}
}

func TestPseudonymizeReporter(t *testing.T) {
	k := []byte("key")
	a := PseudonymizeReporter(k, "t1", "u1")
	if a != PseudonymizeReporter(k, "t1", "u1") {
		t.Error("must be deterministic")
	}
	if a == PseudonymizeReporter(k, "t2", "u1") || a == PseudonymizeReporter(k, "t1", "u2") || a == PseudonymizeReporter([]byte("other"), "t1", "u1") {
		t.Error("tenant, reporter and key must all matter")
	}
	if len(a) != 36 || a[14] != '4' || a == "u1" {
		t.Errorf("not a v4-shaped UUID: %s", a)
	}
}

// Text columns for every table must be declared erasable or exempt; here we check the lists themselves do not overlap.
func TestErasableAndExemptDoNotOverlap(t *testing.T) {
	for _, c := range ErasableColumns {
		if _, dup := ExemptTextColumns[c.Table+"."+c.Column]; dup {
			t.Errorf("%s.%s is both erasable and exempt", c.Table, c.Column)
		}
		if c.KeyColumn == "" {
			t.Errorf("%s.%s has no key column", c.Table, c.Column)
		}
	}
	for k, why := range ExemptTextColumns {
		if why == "" {
			t.Errorf("%s exempt without a reason", k)
		}
	}
}
