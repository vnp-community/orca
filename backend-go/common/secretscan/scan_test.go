package secretscan

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type Vector struct {
	Name         string   `json:"name"`
	Input        string   `json:"input"`
	WantRedacted string   `json:"want_redacted"`
	Kinds        []string `json:"kinds"`
	Confidence   string   `json:"confidence"`
	IsNegative   bool     `json:"is_negative"`
}

func TestVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("failed to read test vectors: %v", err)
	}
	var vectors []Vector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("failed to parse vectors: %v", err)
	}

	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			got, changed := Redact(v.Input)
			if v.IsNegative {
				if changed || got != v.Input {
					t.Errorf("Negative vector should not change. Got: %s", got)
				}
				return
			}

			if got != v.WantRedacted {
				t.Errorf("Redact mismatch. Want: %s, Got: %s", v.WantRedacted, got)
			}

			// Test RedactKinds
			res := RedactKinds(v.Input, Confidence(v.Confidence))
			if len(res.Kinds) != len(v.Kinds) {
				t.Errorf("Kinds mismatch length. Want: %v, Got: %v", v.Kinds, res.Kinds)
			} else {
				for i, k := range v.Kinds {
					if string(res.Kinds[i]) != k {
						t.Errorf("Kind mismatch at %d. Want: %s, Got: %s", i, k, res.Kinds[i])
					}
				}
			}
		})
	}
}

func TestNegativeVectorsUntouched(t *testing.T) {
	inputs := []string{
		"id=3f2504e0-4f89-11d3-9a0c-0305e82c3301",
		"hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"password policy says no",
		"sk-short",
		"user@example.com",
		"https://example.com/a?x=1",
		"tokenCount := 5",
	}

	for _, in := range inputs {
		out, changed := Redact(in)
		if changed || out != in {
			t.Errorf("Negative vector %q was altered to %q", in, out)
		}
	}
}

func TestAnthropicBeforeOpenAI(t *testing.T) {
	in := "sk-ant-12345678901234567890"
	res := RedactKinds(in, ConfidenceHigh)
	if len(res.Kinds) != 1 || res.Kinds[0] != KindAnthropicKey {
		t.Errorf("Expected Anthropic key, got %v", res.Kinds)
	}
}

func TestPrivateKeyBlockWholeBlock(t *testing.T) {
	in := "some text\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\nsome private stuff\nAnd no end block"
	got, _ := Redact(in)
	want := "some text\n[REDACTED]"
	if got != want {
		t.Errorf("Expected %q, got %q", want, got)
	}
}

func TestRedactKindsMinConfidenceHigh(t *testing.T) {
	in := "Bearer abcdef123456 and ghp_abcdefghijklmnopqrstuvwxyz"
	res := RedactKinds(in, ConfidenceHigh)
	if len(res.Kinds) != 1 || res.Kinds[0] != KindGitHubToken {
		t.Errorf("Expected only high confidence github token, got %v", res.Kinds)
	}
}

func TestLargeInputLinear(t *testing.T) {
	large := strings.Repeat("a", 2*1024*1024) + " ghp_abcdefghijklmnopqrstuvwxyz "
	// Should process fast and truncate processing to 1MB, leaving the token untouched (since it's beyond 1MB)
	res := RedactKinds(large, ConfidenceHigh)
	if res.Text != large {
		t.Errorf("Expected text to be unchanged (token beyond 1MB limit)")
	}
	if !res.Truncated {
		t.Errorf("Expected truncated flag")
	}
}

func TestNoValueInFindings(t *testing.T) {
	findings := Scan("ghp_abcdefghijklmnopqrstuvwxyz")
	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding")
	}
	// Finding struct has only Kind, Confidence, Start, End
	_ = Finding{Kind: KindGitHubToken, Confidence: ConfidenceHigh, Start: 0, End: 30}
}

func TestIdempotent(t *testing.T) {
	in := "Token ghp_abcdefghijklmnopqrstuvwxyz"
	out1, _ := Redact(in)
	out2, changed := Redact(out1)
	if changed {
		t.Errorf("Second pass should not change anything")
	}
	if out1 != out2 {
		t.Errorf("Not idempotent")
	}
}

func FuzzRedact(f *testing.F) {
	f.Add("Bearer abcdef123456")
	f.Add("ghp_abcdefghijklmnopqrstuvwxyz")
	f.Fuzz(func(t *testing.T, s string) {
		out, _ := Redact(s)
		if len(out) > len(s) + 64*10 { // bounded growth
			t.Errorf("Output length %d too large for input length %d", len(out), len(s))
		}
	})
}
