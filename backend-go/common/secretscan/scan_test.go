package secretscan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type vector struct {
	Name         string   `json:"name"`
	Input        string   `json:"input"`
	WantRedacted string   `json:"want_redacted"`
	Kinds        []string `json:"kinds"`
	Confidence   string   `json:"confidence"`
	IsNegative   bool     `json:"is_negative"`
}

func loadVectors(t testing.TB) []vector {
	t.Helper()
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vs []vector
	if err := json.Unmarshal(data, &vs); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return vs
}

func TestVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			got, changed := Redact(v.Input)
			if got != v.WantRedacted {
				t.Fatalf("Redact mismatch.\nwant: %q\n got: %q", v.WantRedacted, got)
			}
			if changed != (len(v.Kinds) > 0) {
				t.Fatalf("changed=%v but kinds=%v", changed, v.Kinds)
			}
			res := RedactKinds(v.Input, ConfidenceMedium)
			var kinds []string
			for _, k := range res.Kinds {
				kinds = append(kinds, string(k))
			}
			if !reflect.DeepEqual(kinds, v.Kinds) && (len(kinds) != 0 || len(v.Kinds) != 0) {
				t.Fatalf("kinds want %v got %v", v.Kinds, kinds)
			}
			if len(v.Kinds) == 1 {
				want := strings.ReplaceAll(v.WantRedacted, redactedMarker, "[REDACTED:"+v.Kinds[0]+"]")
				if res.Text != want {
					t.Fatalf("RedactKinds text want %q got %q", want, res.Text)
				}
				fs := Scan(v.Input)
				if len(fs) != 1 || string(fs[0].Confidence) != v.Confidence {
					t.Fatalf("confidence want %s got %+v", v.Confidence, fs)
				}
				if !utf8.ValidString(res.Text) {
					t.Fatalf("output is not valid UTF-8")
				}
			}
		})
	}
}

// Guards the shared contract: every kind has at least two positive vectors and the
// required carriers (Vietnamese text, JSON, URL) plus the required negatives exist.
func TestVectorsCoverage(t *testing.T) {
	vs := loadVectors(t)
	perKind := map[string]int{}
	var neg, viet, js, url int
	for _, v := range vs {
		for _, k := range v.Kinds {
			perKind[k]++
		}
		if v.IsNegative {
			neg++
			continue
		}
		if strings.ContainsAny(v.Input, "ăâêôơưđáàảãạéèẻẽẹíìỉĩịóòỏõọúùủũụýỳỷỹỵ") {
			viet++
		}
		if strings.Contains(v.Input, `{"`) {
			js++
		}
		if strings.Contains(v.Input, "://") {
			url++
		}
	}
	for _, k := range []Kind{KindPrivateKey, KindGitHubToken, KindAWSAccessKey, KindBearer, KindOpenAIKey,
		KindAnthropicKey, KindSlackToken, KindJWT, KindGoogleAPIKey, KindVaultToken, KindConnectionString,
		KindDotenvSecret, KindSecretAssignment} {
		if perKind[string(k)] < 2 {
			t.Errorf("kind %s has %d positive vectors, want >= 2", k, perKind[string(k)])
		}
	}
	if neg < 8 || viet < 3 || js < 3 || url < 3 {
		t.Errorf("coverage too thin: negatives=%d vietnamese=%d json=%d url=%d", neg, viet, js, url)
	}
}

func TestNegativeVectorsUntouched(t *testing.T) {
	for _, v := range loadVectors(t) {
		if !v.IsNegative {
			continue
		}
		if out, changed := Redact(v.Input); changed || out != v.Input {
			t.Errorf("%s: negative vector altered to %q", v.Name, out)
		}
		if res := RedactKinds(v.Input, ConfidenceMedium); res.Text != v.Input || len(res.Kinds) != 0 {
			t.Errorf("%s: RedactKinds altered negative vector: %+v", v.Name, res)
		}
	}
}

func TestAnthropicBeforeOpenAI(t *testing.T) {
	res := RedactKinds("sk-ant-12345678901234567890", ConfidenceHigh)
	if len(res.Kinds) != 1 || res.Kinds[0] != KindAnthropicKey {
		t.Fatalf("want anthropic_key, got %v", res.Kinds)
	}
}

func TestPrivateKeyBlockWholeBlock(t *testing.T) {
	in := "some text\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\nsome private stuff\nAnd no end block"
	if got, _ := Redact(in); got != "some text\n[REDACTED]" {
		t.Fatalf("unterminated block: got %q", got)
	}
	in = "a\n-----BEGIN PRIVATE KEY-----\nAAA\n-----END PRIVATE KEY-----\nb"
	if got, _ := Redact(in); got != "a\n[REDACTED]\nb" {
		t.Fatalf("terminated block: got %q", got)
	}
}

func TestRedactKindsMinConfidenceHigh(t *testing.T) {
	in := "Bearer abcdef123456 and ghp_abcdefghijklmnopqrstuvwxyz and password=hunter2hunter2"
	res := RedactKinds(in, ConfidenceHigh)
	if len(res.Kinds) != 1 || res.Kinds[0] != KindGitHubToken {
		t.Fatalf("want only github_token, got %v", res.Kinds)
	}
	if !strings.Contains(res.Text, "Bearer abcdef123456") || !strings.Contains(res.Text, "hunter2hunter2") {
		t.Fatalf("medium findings must stay at high threshold: %q", res.Text)
	}
	if strings.Contains(res.Text, "ghp_") {
		t.Fatalf("high finding leaked: %q", res.Text)
	}
}

func TestSpecificTokenBeatsGenericAssignment(t *testing.T) {
	in := "token: ghp_abcdefghijklmnopqrstuvwxyz@host"
	fs := Scan(in)
	if len(fs) != 1 || fs[0].Kind != KindGitHubToken {
		t.Fatalf("want one github_token finding, got %+v", fs)
	}
	if got, _ := Redact(in); got != "token: [REDACTED]@host" {
		t.Fatalf("got %q", got)
	}
}

func TestFindingOffsetsCoverOnlyTheValue(t *testing.T) {
	in := "dsn postgres://bob:s3cretpw@db:5432/x và password=hunter2hunter2"
	fs := Scan(in)
	if len(fs) != 2 {
		t.Fatalf("want 2 findings, got %+v", fs)
	}
	if in[fs[0].Start:fs[0].End] != "s3cretpw" || in[fs[1].Start:fs[1].End] != "hunter2hunter2" {
		t.Fatalf("offsets wrong: %+v", fs)
	}
}

func TestEmptyAndTinyInputs(t *testing.T) {
	if out, changed := Redact(""); out != "" || changed {
		t.Fatalf("empty: %q %v", out, changed)
	}
	if res := RedactKinds("", ConfidenceHigh); res.Text != "" || res.Truncated || len(res.Kinds) != 0 {
		t.Fatalf("empty RedactKinds: %+v", res)
	}
	if Scan("") != nil {
		t.Fatal("empty Scan must be nil")
	}
}

func TestLargeInputLinear(t *testing.T) {
	token := "ghp_abcdefghijklmnopqrstuvwxyz"
	pad := strings.Repeat("lorem ipsum dolor sit amet ", 40000)[:1024*1024-100]
	large := pad + " " + token + " " + strings.Repeat("a", 1024*1024)
	start := time.Now()
	res := RedactKinds(large, ConfidenceHigh)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("2 MiB took %v, want < 1s", d)
	}
	if !res.Truncated {
		t.Fatal("expected Truncated for > 1 MiB input")
	}
	if len(res.Kinds) != 1 || strings.Contains(res.Text[:1024*1024], "ghp_") {
		t.Fatalf("token inside the scan window must be redacted: kinds=%v", res.Kinds)
	}
	if !strings.HasSuffix(res.Text, strings.Repeat("a", 1024*1024)) {
		t.Fatal("tail past the window must be kept verbatim")
	}
	// Past the window nothing is scanned; the flag tells the caller.
	beyond := strings.Repeat("a", 2*1024*1024) + " " + token
	if r := RedactKinds(beyond, ConfidenceHigh); r.Text != beyond || !r.Truncated {
		t.Fatal("token beyond the window stays and Truncated is set")
	}
}

func TestNoValueInFindings(t *testing.T) {
	secret := "ghp_abcdefghijklmnopqrstuvwxyz"
	typ := reflect.TypeOf(Finding{})
	for i := 0; i < typ.NumField(); i++ {
		if k := typ.Field(i).Type.Kind(); k == reflect.String && typ.Field(i).Type != reflect.TypeOf(KindGitHubToken) && typ.Field(i).Type != reflect.TypeOf(ConfidenceHigh) {
			t.Fatalf("Finding field %s could hold a value", typ.Field(i).Name)
		}
	}
	fs := Scan("x " + secret)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	if s := fmt.Sprintf("%+v %v", fs, RedactKinds("x "+secret, ConfidenceHigh).Kinds); strings.Contains(s, secret) {
		t.Fatalf("value leaked in %s", s)
	}
}

func TestIdempotent(t *testing.T) {
	for _, v := range loadVectors(t) {
		once, _ := Redact(v.Input)
		twice, changed := Redact(once)
		if changed || twice != once {
			t.Errorf("%s: not idempotent: %q -> %q", v.Name, once, twice)
		}
		k1 := RedactKinds(v.Input, ConfidenceMedium).Text
		if k2 := RedactKinds(k1, ConfidenceMedium); k2.Text != k1 || len(k2.Kinds) != 0 {
			t.Errorf("%s: RedactKinds not idempotent: %q -> %q", v.Name, k1, k2.Text)
		}
	}
}

func TestManyFindingsStayFast(t *testing.T) {
	in := strings.Repeat("password=hunter2hunter2 ghp_abcdefghijklmnopqrstuvwxyz\n", 8000)
	start := time.Now()
	out, changed := Redact(in)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("many findings took %v", d)
	}
	if !changed || strings.Contains(out, "ghp_") || strings.Contains(out, "hunter2") {
		t.Fatal("not fully redacted")
	}
}

// Bumping PatternsVersion is mandatory when the table changes: update both constants together.
func TestPatternsVersionMatchesTable(t *testing.T) {
	h := sha256.New()
	for _, p := range secretPatterns {
		fmt.Fprintf(h, "%s|%s|%d|%s\n", p.kind, p.confidence, p.valueGroup, p.re.String())
	}
	got := hex.EncodeToString(h.Sum(nil))
	const wantHash = "9a5894ea17a5c49d3efccdddeb6286b9b178cee150a9de7a1f7ecc9377add1be"
	if PatternsVersion != "ss/1" || got != wantHash {
		t.Fatalf("pattern table changed: bump PatternsVersion and update wantHash to %s", got)
	}
}

func FuzzRedact(f *testing.F) {
	for _, v := range loadVectors(f) {
		f.Add(v.Input)
	}
	f.Add("Bearer abcdef123456")
	f.Add("-----BEGIN PRIVATE KEY-----")
	f.Fuzz(func(t *testing.T, s string) {
		out, changed := Redact(s)
		n := len(Scan(s))
		if len(out) > len(s)+64*n {
			t.Fatalf("output %d too large for input %d with %d findings", len(out), len(s), n)
		}
		if !changed && out != s {
			t.Fatal("unchanged flag lies")
		}
		if utf8.ValidString(s) && !utf8.ValidString(out) {
			t.Fatal("output must stay valid UTF-8")
		}
		RedactKinds(s, ConfidenceHigh)
	})
}
