package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

type canonicalGoldenCase struct {
	Name      string `json:"name"`
	Input     string `json:"input"`
	Canonical string `json:"canonical"`
	Digest    string `json:"digest"`
}

// The golden file is byte-identical with task-service's copy so both services hash alike.
func TestCanonicalJSON_Golden(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/artifacts/task/canonical_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []canonicalGoldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 5 {
		t.Fatalf("golden file too small: %d", len(cases))
	}
	for _, c := range cases {
		got, err := CanonicalJSON([]byte(c.Input))
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if string(got) != c.Canonical {
			t.Errorf("%s: canonical = %s, want %s", c.Name, got, c.Canonical)
		}
		if d := DigestOfCanonical(got); d != c.Digest {
			t.Errorf("%s: digest = %s, want %s", c.Name, d, c.Digest)
		}
	}
}

func TestCanonicalJSON_Rejections(t *testing.T) {
	deep := strings.Repeat("[", 40) + strings.Repeat("]", 40)
	for name, in := range map[string]string{
		"duplicate key": `{"a":1,"a":2}`,
		"trailing":      `{"a":1} {"b":2}`,
		"too deep":      deep,
		"invalid":       `{"a":`,
	} {
		if _, err := CanonicalJSON([]byte(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestCanonicalJSON_Idempotent(t *testing.T) {
	in := `{"b":[1,2.50,{"z":"x","a":true}],"a":"Nguyễn"}`
	once, err := CanonicalJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := CanonicalJSON(once)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Fatalf("not idempotent: %s vs %s", once, twice)
	}
}

func TestCanonicalJSON_NFC_ComposedAndDecomposedSameDigest(t *testing.T) {
	composed := norm.NFC.String("Nguyễn Văn Đức")
	decomposed := norm.NFD.String(composed)
	if composed == decomposed {
		t.Fatal("test strings must differ byte-wise")
	}
	d1, err := DigestValue(map[string]string{"name": composed})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := DigestValue(map[string]string{"name": decomposed})
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("digests differ: %s vs %s", d1, d2)
	}
}

func TestDigestOptions_UsesCanonicalCore(t *testing.T) {
	// A duplicate key used to be swallowed silently; the shared core now refuses it.
	if _, err := DigestOptions([]byte(`{"a":1,"a":2}`), nil); err == nil {
		t.Fatal("duplicate key must be rejected")
	}
	chosen := 0
	composed, _ := DigestOptions([]byte(`{"t":"Nguyễn"}`), &chosen)
	decomposed, _ := DigestOptions([]byte("{\"t\":\""+norm.NFD.String("Nguyễn")+"\"}"), &chosen)
	if composed != decomposed {
		t.Fatal("DigestOptions must normalise NFC through CanonicalJSON")
	}
}

func FuzzCanonicalJSON(f *testing.F) {
	for _, s := range []string{`{}`, `[]`, `{"a":1}`, `{"b":[1,2,{"c":null}]}`, `"x"`, `1.5e3`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		once, err := CanonicalJSON(in)
		if err != nil {
			return
		}
		twice, err := CanonicalJSON(once)
		if err != nil {
			t.Fatalf("canonical output must re-parse: %v", err)
		}
		if string(once) != string(twice) {
			t.Fatalf("not idempotent: %q vs %q", once, twice)
		}
	})
}
