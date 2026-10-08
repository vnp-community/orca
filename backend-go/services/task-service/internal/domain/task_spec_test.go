package domain

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestNewTaskSpec_Valid(t *testing.T) {
	s, err := NewTaskSpec("t1", "ten", 2, []byte(`{ "b": 1, "a": "x" }`))
	if err != nil {
		t.Fatal(err)
	}
	if string(s.Spec) != `{"a":"x","b":1}` || len(s.Digest) != 64 || s.IsLocked() {
		t.Fatalf("unexpected spec: %+v", s)
	}
}

func TestNewTaskSpec_InvalidJSON(t *testing.T) {
	for _, in := range []string{``, `{`, `[1]`, `"x"`, `null`, `{"a":1,"a":2}`} {
		if _, err := NewTaskSpec("t1", "ten", 1, []byte(in)); !errors.Is(err, ErrTaskSpecInvalid) {
			t.Errorf("%q: want ErrTaskSpecInvalid, got %v", in, err)
		}
	}
	if _, err := NewTaskSpec("t1", "ten", 0, []byte(`{}`)); !errors.Is(err, ErrTaskSpecInvalid) {
		t.Errorf("schema_version 0 accepted")
	}
}

func TestNewTaskSpec_TooLarge(t *testing.T) {
	pad := strings.Repeat("x", MaxTaskSpecBytes-len(`{"a":""}`))
	if _, err := NewTaskSpec("t1", "ten", 1, []byte(`{"a":"`+pad+`"}`)); err != nil {
		t.Fatalf("exactly 32 KB must pass: %v", err)
	}
	if _, err := NewTaskSpec("t1", "ten", 1, []byte(`{"a":"`+pad+`x"}`)); !errors.Is(err, ErrTaskSpecInvalid) {
		t.Fatalf("32 KB + 1 must fail, got %v", err)
	}
}

func TestNewTaskSpec_DigestStableAcrossKeyOrder(t *testing.T) {
	a, _ := NewTaskSpec("t1", "ten", 1, []byte(`{"a":1,"b":{"c":2,"d":3}}`))
	b, _ := NewTaskSpec("t1", "ten", 1, []byte(`{"b":{"d":3,"c":2},"a":1.0}`))
	if a.Digest != b.Digest {
		t.Fatalf("digests differ: %s vs %s", a.Digest, b.Digest)
	}
}

func TestNewTaskSpec_VietnameseNFC(t *testing.T) {
	a, _ := NewTaskSpec("t1", "ten", 1, []byte(`{"title":"`+norm.NFC.String("Nguyễn Văn")+`"}`))
	b, _ := NewTaskSpec("t1", "ten", 1, []byte(`{"title":"`+norm.NFD.String("Nguyễn Văn")+`"}`))
	if a.Digest != b.Digest {
		t.Fatal("composed and decomposed Vietnamese must hash alike")
	}
}
