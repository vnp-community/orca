package usecase

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestValidateByKind_AcceptsGoldenAndRejectsBroken(t *testing.T) {
	good := map[domain.SolutionKind]string{
		domain.SolutionKindSolution:  validSolutionReply(t),
		domain.SolutionKindDiagnosis: analysisReply(t, "diagnosis_valid.json"),
		domain.SolutionKindFindings:  analysisReply(t, "findings_valid.json"),
		domain.SolutionKindAnswer:    analysisReply(t, "answer_valid.json"),
	}
	for kind, doc := range good {
		out, err := ValidateByKind(kind, []byte(doc), 2)
		if err != nil || len(out) == 0 {
			t.Errorf("%s: %v", kind, err)
		}
	}
	bad := map[domain.SolutionKind]string{
		domain.SolutionKindSolution:  oneOptionReply(),
		domain.SolutionKindDiagnosis: analysisReply(t, "diagnosis_invalid_no_statement.json"),
		domain.SolutionKindFindings:  analysisReply(t, "findings_invalid_no_findings.json"),
		domain.SolutionKindAnswer:    analysisReply(t, "answer_invalid_empty.json"),
	}
	for kind, doc := range bad {
		if _, err := ValidateByKind(kind, []byte(doc), 2); err == nil {
			t.Errorf("%s: broken document accepted", kind)
		}
	}
	// A document of the wrong kind must not pass as another kind.
	if _, err := ValidateByKind(domain.SolutionKindAnswer, []byte(good[domain.SolutionKindFindings]), 2); err == nil {
		t.Error("findings accepted as answer")
	}
	if _, err := ValidateByKind("mystery", []byte(`{}`), 1); err == nil {
		t.Error("unknown kind accepted")
	}
}

func TestRedactSecrets_PositiveAndNegativeSamples(t *testing.T) {
	positive := map[string]string{
		"pem":   "x\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\ny",
		"ghp":   "token ghp_abcdefghijklmnopqrstuvwxyz0123456789 end",
		"aws":   "key AKIAIOSFODNN7EXAMPLE end",
		"jwt":   "auth eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVPmB92K27uhbUJU1p1r_wW1gFWFOEjXk end",
		"pwd":   "config password=hunter2secret ok",
		"pwdUC": "PASSWORD: hunter2secret ok",
	}
	for name, in := range positive {
		out, n := RedactSecrets(in)
		if n < 1 || !strings.Contains(out, "[REDACTED]") {
			t.Errorf("%s: not redacted: %q (n=%d)", name, out, n)
		}
	}
	negative := []string{
		"AKIA ngắn", "mật khẩu: password không có giá trị", "Văn bản tiếng Việt bình thường, có dấu: đường dẫn internal/usecase/profile.go:42",
		"ghp_short",
	}
	for _, in := range negative {
		if out, n := RedactSecrets(in); n != 0 || out != in {
			t.Errorf("false positive on %q -> %q (n=%d)", in, out, n)
		}
	}
	out, _ := RedactSecrets("pass password=hunter2secret ế")
	if !strings.HasSuffix(out, " ế") {
		t.Errorf("multi-byte text damaged: %q", out)
	}
}

func TestRedactDocument_WalksEveryStringAndKeepsTheShape(t *testing.T) {
	doc := withLeaks(analysisReply(t, "diagnosis_valid.json"))
	out, n, err := RedactDocument([]byte(doc))
	if err != nil || n < 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if strings.Contains(string(out), "ghp_abcdef") || strings.Contains(string(out), "PRIVATE KEY") {
		t.Fatalf("secret survived: %s", out)
	}
	if _, err := ValidateByKind(domain.SolutionKindDiagnosis, out, 2); err != nil {
		t.Fatalf("redacted document no longer valid: %v", err)
	}
	if _, _, err := RedactDocument([]byte("not json")); err == nil {
		t.Fatal("garbage must error")
	}
}

func TestRedactRaw_BoundsInputBeforeScanning(t *testing.T) {
	big := strings.Repeat("a", 300*1024)
	if got := RedactRaw(big); len(got) > 256*1024 {
		t.Fatalf("raw output not truncated: %d", len(got))
	}
}
