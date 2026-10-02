package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func hash(t *testing.T, args string) string {
	t.Helper()
	h, err := ParamsHash("t1", "u1", "c1", "terminal.send", []byte(args))
	if err != nil {
		t.Fatalf("%s: %v", args, err)
	}
	return h
}

func TestParamsHashCanonicalization(t *testing.T) {
	base := hash(t, `{"a":1,"b":[1,2,{"x":"y"}],"c":"d"}`)
	if !strings.HasPrefix(base, "sha256:") || len(base) != len("sha256:")+64 {
		t.Fatalf("format: %s", base)
	}
	same := []string{
		`{"c":"d","b":[1,2,{"x":"y"}],"a":1}`,
		"{ \"a\" : 1 ,\n \"b\" : [ 1 , 2 , { \"x\" : \"y\" } ] , \"c\" : \"d\" }",
	}
	for _, s := range same {
		if hash(t, s) != base {
			t.Errorf("key order/whitespace must not matter: %s", s)
		}
	}
	different := []string{
		`{"a":1.0,"b":[1,2,{"x":"y"}],"c":"d"}`, // literal numbers are kept: 1 != 1.0
		`{"a":1,"b":[2,1,{"x":"y"}],"c":"d"}`,   // array order matters
		`{"a":1,"b":[1,2,{"x":"y"}],"c":"e"}`,
	}
	for _, s := range different {
		if hash(t, s) == base {
			t.Errorf("must differ: %s", s)
		}
	}
	if hash(t, ``) != hash(t, `{}`) || hash(t, `null`) != hash(t, `{}`) {
		t.Error("absent arguments hash as {}")
	}
	other := []struct{ tenant, user, client, tool string }{{"t2", "u1", "c1", "terminal.send"}, {"t1", "u2", "c1", "terminal.send"}, {"t1", "u1", "c2", "terminal.send"}, {"t1", "u1", "c1", "terminal.read"}}
	for _, o := range other {
		h, _ := ParamsHash(o.tenant, o.user, o.client, o.tool, []byte(`{}`))
		if h == hash(t, `{}`) {
			t.Errorf("hash must bind %+v", o)
		}
	}
}

func TestParamsHashFixedVector(t *testing.T) {
	// canonical = {"args":{"cmd":"ls"},"client":"c1","tenant":"t1","tool":"terminal.send","user":"u1","v":1}
	got := hash(t, `{"cmd":"ls"}`)
	want := "sha256:" + sha256Hex(`{"args":{"cmd":"ls"},"client":"c1","tenant":"t1","tool":"terminal.send","user":"u1","v":1}`)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestParamsHashEscapingIsNotHTMLAndKeepsControls(t *testing.T) {
	a := hash(t, `{"s":"a<b&c>"}`)
	want := "sha256:" + sha256Hex(`{"args":{"s":"a<b&c>"},"client":"c1","tenant":"t1","tool":"terminal.send","user":"u1","v":1}`)
	if a != want {
		t.Fatal("< > & must not be escaped")
	}
	nl := hash(t, `{"s":"a\nb"}`)
	want = "sha256:" + sha256Hex(`{"args":{"s":"a`+`\`+`u000ab"},"client":"c1","tenant":"t1","tool":"terminal.send","user":"u1","v":1}`)
	if nl != want {
		t.Fatal("control characters are escaped as lowercase \\u00xx")
	}
}

func TestParamsHashRejectsAmbiguousInput(t *testing.T) {
	bad := []string{`{"cmd":"ls","cmd":"rm"}`, `{"a":{"b":1,"b":2}}`, `[1,2]`, `"x"`, `{"a":1} {"b":2}`, `{"a":`, strings.Repeat("[", 40) + strings.Repeat("]", 40)}
	for _, b := range bad {
		if _, err := ParamsHash("t", "u", "c", "x", []byte(b)); err == nil {
			t.Errorf("must reject %q", b)
		}
	}
	if _, err := ParamsHash("t", "u", "c", "x", []byte(`{"a":1,"a":2}`)); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("duplicate key error: %v", err)
	}
}

func TestParamsHashEqualConstantTime(t *testing.T) {
	if !ParamsHashEqual("sha256:ab", "sha256:ab") || ParamsHashEqual("sha256:ab", "sha256:ac") || ParamsHashEqual("a", "") {
		t.Fatal("equality")
	}
}

func TestArgsPreviewRedactsAndRevealsInvisible(t *testing.T) {
	raw := `{"z":1,"a":"token=ghp_abcdefghijklmnopqrstuvwxyz0123456789 ` + string(rune(0x202e)) + `x"}`
	text, redacted, err := ArgsPreview([]byte(raw), SecretRedactor{})
	if err != nil || !redacted {
		t.Fatalf("%v redacted=%v", err, redacted)
	}
	if strings.Contains(text, "ghp_abcdef") || strings.ContainsRune(text, rune(0x202e)) || !strings.Contains(text, "\\"+"u202E") {
		t.Fatalf("preview: %q", text)
	}
	if strings.Index(text, `"a"`) > strings.Index(text, `"z"`) {
		t.Fatal("keys must be sorted")
	}
}

func TestSecretRedactor(t *testing.T) {
	r := SecretRedactor{}
	secrets := []string{
		"ghp_abcdefghijklmnopqrstuvwxyz0123456789", "github_pat_11ABCDEFG0123456789_abcdefghijklmnop", "AKIAIOSFODNN7EXAMPLE",
		"Bearer abcdefghijklmnop", "sk-abcdefghijklmnopqrstuvwxyz", "xoxb-1234567890-abcdefghij",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghijklmnop",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----",
		`password: "hunter2hunter2"`, "api_key=abcd1234", "Authorization: abcdefgh",
	}
	for _, s := range secrets {
		out, changed := r.Redact("before " + s + " after")
		if !changed || !strings.Contains(out, "[REDACTED]") || strings.Contains(out, "hunter2hunter2") {
			t.Errorf("not redacted: %q -> %q", s, out)
		}
	}
	if out, changed := r.Redact("just a normal command: ls -la /tmp"); changed || out != "just a normal command: ls -la /tmp" {
		t.Fatalf("false positive: %q", out)
	}
}

func TestValidateEgressArgs(t *testing.T) {
	r := SecretRedactor{}
	bad := []string{
		`{"u":"https://user:pw@host.example/x"}`, `{"u":"https://h.example/?token=abc"}`, `{"u":"see https://h.example/?api_key=abc now"}`,
		`{"nested":{"k":["AKIAIOSFODNN7EXAMPLE"]}}`, `{"u":"https://h.example/?Signature=zzz"}`,
	}
	for _, b := range bad {
		if err := ValidateEgressArgs([]byte(b), r); !errors.Is(err, ErrEgressSecret) {
			t.Errorf("must block %s (got %v)", b, err)
		}
	}
	for _, g := range []string{`{"u":"https://example.com/page?id=7"}`, `{"text":"hello"}`, `{}`} {
		if err := ValidateEgressArgs([]byte(g), r); err != nil {
			t.Errorf("must allow %s: %v", g, err)
		}
	}
}

func TestApprovalStateMachine(t *testing.T) {
	allowed := map[[2]string]bool{
		{ApprovalPending, ApprovalApproved}: true, {ApprovalPending, ApprovalDenied}: true, {ApprovalPending, ApprovalExpired}: true,
		{ApprovalPending, ApprovalCancelled}: true, {ApprovalApproved, ApprovalExpired}: true, {ApprovalApproved, ApprovalCancelled}: true,
	}
	all := []string{ApprovalPending, ApprovalApproved, ApprovalDenied, ApprovalExpired, ApprovalCancelled}
	for _, from := range all {
		for _, to := range all {
			if CanTransition(from, to) != allowed[[2]string{from, to}] {
				t.Errorf("%s -> %s", from, to)
			}
		}
	}
}

func TestDecideDiagnosisOrder(t *testing.T) {
	now := time.Now()
	pending := Approval{Status: ApprovalPending, ExpiresAt: now.Add(time.Minute)}
	cases := []struct {
		name  string
		a     Approval
		found bool
		code  string
	}{
		{"missing or foreign", Approval{}, false, CodeNotFound},
		{"expired beats decided", Approval{Status: ApprovalApproved, ExpiresAt: now.Add(-time.Second)}, true, ""},
		{"pending past deadline", Approval{Status: ApprovalPending, ExpiresAt: now.Add(-time.Second)}, true, CodeApprovalExpired},
		{"already decided", Approval{Status: ApprovalDenied, ExpiresAt: now.Add(time.Minute)}, true, CodeApprovalAlreadyDecided},
		{"hash mismatch", pending, true, CodeApprovalHashMismatch},
	}
	for _, c := range cases {
		if c.code == "" {
			continue
		}
		err := DecideDiagnosis(c.a, c.found, now)
		if err == nil || !strings.Contains(err.Error(), c.code) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if (Approval{Status: ApprovalPending, ExpiresAt: now.Add(-time.Second)}).EffectiveStatus(now) != ApprovalExpired {
		t.Fatal("effective status must apply expiry")
	}
}

func TestToolPolicyValidate(t *testing.T) {
	ok := ToolPolicy{Decision: DecisionDeny, Match: ToolPolicyMatch{Tool: "x"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []ToolPolicy{
		{Decision: "nope", Match: ToolPolicyMatch{Tool: "x"}}, {Decision: DecisionDeny},
		{Decision: DecisionDeny, Match: ToolPolicyMatch{Risk: "bogus"}}, {Decision: DecisionDeny, Match: ToolPolicyMatch{Roles: []string{"root"}}},
		{Decision: DecisionDeny, Match: ToolPolicyMatch{Tool: "x"}, Note: strings.Repeat("n", 501)},
	}
	for i, p := range bad {
		if p.Validate() == nil {
			t.Errorf("case %d must be invalid", i)
		}
	}
	if _, has := (ToolPolicyMatch{Tool: "x"}).AsMap()["namespace"]; has {
		t.Fatal("unset dimensions must be omitted from the Rego input")
	}
}

func TestParseDecisionFailsClosed(t *testing.T) {
	for _, m := range []map[string]any{nil, {}, {"decision": "maybe"}, {"decision": 7}} {
		if ParseDecision(m).Decision != DecisionDeny {
			t.Errorf("%v must deny", m)
		}
	}
	if d := ParseDecision(map[string]any{"decision": "allow", "source": "default", "reasons": []any{"a"}}); d.Decision != DecisionAllow || len(d.Reasons) != 1 {
		t.Fatalf("%+v", d)
	}
}

func TestKillStateBlocked(t *testing.T) {
	st := KillState{Entries: []KillSwitchEntry{{Scope: KillScopeClient, TargetID: "c1", Active: true}, {Scope: KillScopeSession, TargetID: "s1", Active: true}, {Scope: KillScopeGrant, TargetID: "g1", Active: false}}}
	if _, b := st.Blocked("c1", "", ""); !b {
		t.Error("client")
	}
	if _, b := st.Blocked("c2", "", "s1"); !b {
		t.Error("session")
	}
	if _, b := st.Blocked("c2", "g1", "s2"); b {
		t.Error("inactive grant switch must not block")
	}
	if _, b := (KillState{Entries: []KillSwitchEntry{{Scope: KillScopeTenant, Active: true}}}).Blocked("", "", ""); !b {
		t.Error("tenant")
	}
	e := KillSwitchEntry{Scope: KillScopeClient, Reason: "ok!"}
	if e.Validate() == nil {
		t.Error("client scope needs a target")
	}
	e = KillSwitchEntry{Scope: KillScopeTenant, TargetID: "junk", Reason: "ok!"}
	if err := e.Validate(); err != nil || e.TargetID != "" {
		t.Error("tenant scope drops the target")
	}
	if (&KillSwitchEntry{Scope: KillScopeTenant, Reason: "x"}).Validate() == nil {
		t.Error("reason must be at least 3 characters")
	}
}

func TestAuditEventShapeAndOutcome(t *testing.T) {
	c := ToolCall{ID: "c", TenantID: "t", UserID: "u", ClientID: "cl", ToolName: "terminal_send", Risk: "exec", Decision: CallApproved, ArgsSummary: "x", Result: "ok"}
	ev, err := NewToolCallAuditEvent("e1", c, time.Unix(0, 0), 3)
	if err != nil || ev.Subject != SubjectAuditAppended {
		t.Fatalf("%v %v", ev, err)
	}
	s := string(ev.PayloadJSON)
	for _, want := range []string{`"actor_type":"agent"`, `"action":"mcp.tool_call"`, `"outcome":"allowed"`, `"audit_id":"c"`, `"suppressed_count":3`, `"target_id":"terminal_send"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	for d, want := range map[string]string{CallAllow: "allowed", CallApproved: "allowed", CallDeny: "denied", CallDenied: "denied", CallExpired: "denied"} {
		if AuditOutcome(d) != want {
			t.Errorf("%s", d)
		}
	}
}

func TestSanitizeClientNameAndSummary(t *testing.T) {
	if got := SanitizeClientName("Evil" + string(rune(0x202e)) + "\nName" + strings.Repeat("x", 200)); strings.ContainsRune(got, rune(0x202e)) || strings.Contains(got, "\n") || len([]rune(got)) > 80 {
		t.Fatalf("%q", got)
	}
	if SanitizeClientName("\n\t") != "Unknown client" {
		t.Fatal("empty fallback")
	}
	if got := SummarizeArgs("{\n  \"a\": \"b\"\n}", SecretRedactor{}); got != `{ "a": "b" }` {
		t.Fatalf("%q", got)
	}
	if len([]rune(SummarizeArgs(strings.Repeat("y", 2000), nil))) != MaxArgsSummary {
		t.Fatal("summary cap")
	}
}

func TestRiskClass(t *testing.T) {
	for r, want := range map[string]string{"read": "read", "write_reversible": "write", "exec": "exec", "destructive": "exec", "admin": "exec", "": "exec"} {
		if RiskClass(r) != want {
			t.Errorf("%q", r)
		}
	}
}
