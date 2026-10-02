package mcpscope

import (
	"errors"
	"reflect"
	"testing"
)

func TestCeilingMatrix(t *testing.T) {
	cases := map[string][]string{
		"admin": {Read, Write, Exec, Admin}, "user": {Read, Write, Exec}, "": nil, "root": nil,
	}
	for role, want := range cases {
		if got := CeilingForRole(role); !reflect.DeepEqual(got, want) {
			t.Errorf("role %q: got %v want %v", role, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	got, err := Parse("orca:exec orca:read orca:read")
	if err != nil || !reflect.DeepEqual(got, []string{Read, Exec}) {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := Parse("orca:read orca:bogus"); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("want ErrUnknownScope, got %v", err)
	}
	if got, _ := Parse(""); len(got) != 0 {
		t.Fatalf("empty must parse to empty, got %v", got)
	}
}

func TestIntersectSubset(t *testing.T) {
	if got := Intersect([]string{Read, Admin}, CeilingForRole("user")); !reflect.DeepEqual(got, []string{Read}) {
		t.Fatal(got)
	}
	if Subset([]string{Admin}, CeilingForRole("user")) || !Subset([]string{Read}, CeilingForRole("user")) {
		t.Fatal("subset wrong")
	}
}

func TestImplies(t *testing.T) {
	if !Implies([]string{Write}, "orca:git:write") {
		t.Error("write implies git:write")
	}
	if Implies([]string{Exec}, Write) || Implies([]string{Write}, Exec) || Implies([]string{Admin}, Read) || Implies([]string{Read}, "orca:git:write") {
		t.Error("coarse scopes must not imply each other")
	}
}
