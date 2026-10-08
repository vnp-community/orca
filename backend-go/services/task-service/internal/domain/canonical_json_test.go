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

// The golden file is shared with request-service (CI diffs the two copies) so both services hash alike.
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
	in := `{"z":[1.0,2.50,{"b":true,"a":null}],"a":"Nguyễn"}`
	once, err := CanonicalJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := CanonicalJSON(once)
	if err != nil || string(once) != string(twice) {
		t.Fatalf("not idempotent: %s vs %s (%v)", once, twice, err)
	}
}

func FuzzCanonicalJSON(f *testing.F) {
	f.Add(`{"a":1}`)
	f.Add(`[1.5,"x",null]`)
	f.Fuzz(func(t *testing.T, in string) {
		once, err := CanonicalJSON([]byte(in))
		if err != nil {
			return
		}
		twice, err := CanonicalJSON(once)
		if err != nil || string(once) != string(twice) {
			t.Fatalf("not idempotent for %q", in)
		}
	})
}

func TestCanonicalJSON_NFCKeysAndValues(t *testing.T) {
	composed := norm.NFC.String("Nguyễn")
	decomposed := norm.NFD.String("Nguyễn")
	a, _ := CanonicalJSON([]byte(`{"` + decomposed + `":"` + decomposed + `"}`))
	b, _ := CanonicalJSON([]byte(`{"` + composed + `":"` + composed + `"}`))
	if string(a) != string(b) {
		t.Fatalf("NFC mismatch: %q vs %q", a, b)
	}
}
