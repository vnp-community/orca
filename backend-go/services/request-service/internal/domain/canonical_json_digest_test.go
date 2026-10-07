package domain

import (
	"testing"
)

func TestDigestOptions(t *testing.T) {
	json1 := `{"b":2,"a":1}`
	json2 := `{
		"a": 1,
		"b": 2
	}`
	
	chosen := 1

	d1, err := DigestOptions([]byte(json1), &chosen)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := DigestOptions([]byte(json2), &chosen)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Errorf("digests mismatch: %v != %v", d1, d2)
	}

	chosen2 := 2
	d3, _ := DigestOptions([]byte(json1), &chosen2)
	if d1 == d3 {
		t.Errorf("digests should differ on chosen")
	}

	d4, _ := DigestOptions([]byte(json1), nil)
	if d1 == d4 {
		t.Errorf("digests should differ on nil chosen")
	}
}
