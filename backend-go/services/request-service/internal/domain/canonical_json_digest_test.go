package domain

import (
	"strings"
	"testing"
)

func mustDigest(t *testing.T, doc string, chosen *int) string {
	t.Helper()
	d, err := DigestOptions([]byte(doc), chosen)
	if err != nil {
		t.Fatalf("digest %s: %v", doc, err)
	}
	return d
}

func TestDigestOptions_KeyOrderAndWhitespaceDoNotMatter(t *testing.T) {
	a := mustDigest(t, `{"b":1,"a":{"y":[1,2],"x":"v"}}`, nil)
	b := mustDigest(t, "{\n  \"a\" : {\"x\":\"v\", \"y\":[1, 2]},\n  \"b\": 1\n}", nil)
	if a != b {
		t.Fatalf("digests differ: %s vs %s", a, b)
	}
}

func TestDigestOptions_NumberSpellingIsNormalised(t *testing.T) {
	// JSONB and MySQL JSON rewrite numbers, so 1, 1.0 and 1e0 must agree.
	a := mustDigest(t, `{"h":12.0,"c":0.50}`, nil)
	b := mustDigest(t, `{"h":12,"c":0.5}`, nil)
	c := mustDigest(t, `{"h":1.2e1,"c":5e-1}`, nil)
	if a != b || b != c {
		t.Fatalf("digests differ: %s %s %s", a, b, c)
	}
	if mustDigest(t, `{"h":12}`, nil) == mustDigest(t, `{"h":13}`, nil) {
		t.Fatal("different numbers must differ")
	}
}

func TestDigestOptions_ChosenChangesDigest(t *testing.T) {
	zero, one := 0, 1
	none := mustDigest(t, `{"a":1}`, nil)
	d0 := mustDigest(t, `{"a":1}`, &zero)
	d1 := mustDigest(t, `{"a":1}`, &one)
	if none == d0 || d0 == d1 || none == d1 {
		t.Fatalf("chosen must be part of the digest: %s %s %s", none, d0, d1)
	}
	if d0 != mustDigest(t, `{"a":1}`, &zero) {
		t.Fatal("digest not deterministic")
	}
}

func TestDigestOptions_RejectsDuplicateKeysDepthAndGarbage(t *testing.T) {
	for name, doc := range map[string]string{
		"duplicate":        `{"a":1,"a":2}`,
		"nested duplicate": `{"x":{"a":1,"a":1}}`,
		"trailing":         `{"a":1} {"b":2}`,
		"garbage":          `{"a":`,
		"deep":             strings.Repeat(`{"a":`, 40) + `1` + strings.Repeat(`}`, 40),
	} {
		if _, err := DigestOptions([]byte(doc), nil); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestDigestOptions_VietnameseUnicodeStable(t *testing.T) {
	composed := mustDigest(t, `{"t":"Thêm bộ nhớ đệm"}`, nil)
	escaped := mustDigest(t, `{"t":"Thêm bộ nhớ đệm"}`, nil)
	if composed != escaped {
		t.Fatal("escaped and literal Unicode must hash the same")
	}
}

func TestCanonicalJSON_NoHTMLEscaping(t *testing.T) {
	got, err := CanonicalJSON([]byte(`{"k":"<a>&"}`))
	if err != nil || string(got) != `{"k":"<a>&"}` {
		t.Fatalf("%q %v", got, err)
	}
}
