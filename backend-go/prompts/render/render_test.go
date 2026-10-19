package render

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
)

func set(files map[string]string, funcs template.FuncMap) *Set {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return NewSet(funcs, m)
}

func TestSet_MissingMapKeyIsAnError(t *testing.T) {
	s := set(map[string]string{"a.tmpl": "hello {{.Name}}"}, nil)
	if _, err := s.Execute("a.tmpl", map[string]string{}); err == nil || !strings.Contains(err.Error(), "Name") {
		t.Fatalf("a key absent from the data must fail, got %v", err)
	}
	if got, err := s.Execute("a.tmpl", map[string]string{"Name": "x"}); err != nil || got != "hello x" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSet_UnknownStructFieldIsAnError(t *testing.T) {
	s := set(map[string]string{"a.tmpl": "{{.Nope}}"}, nil)
	if _, err := s.Execute("a.tmpl", struct{ Name string }{"x"}); err == nil {
		t.Fatal("a field the Input does not have must fail")
	}
}

func TestSet_UnknownTemplateAndBrokenFile(t *testing.T) {
	good := set(map[string]string{"a.tmpl": "x"}, nil)
	if _, err := good.Execute("missing.tmpl", nil); err == nil {
		t.Fatal("an unknown template name must fail")
	}
	bad := set(map[string]string{"a.tmpl": "{{if}}"}, nil)
	for i := 0; i < 2; i++ { // the parse error is remembered, not retried into a different result
		if _, err := bad.Execute("a.tmpl", nil); err == nil || !strings.Contains(err.Error(), "parse prompt templates") {
			t.Fatalf("a template that does not parse must fail: %v", err)
		}
	}
}

func TestSet_FuncsFenceAndTruncate(t *testing.T) {
	s := set(map[string]string{"a.tmpl": `{{fence .}}|{{truncate . 4}}|{{closing . "feedback"}}|{{quote .}}|{{inc 1}}|{{join (list) ","}}`},
		template.FuncMap{"list": func() []string { return []string{"a", "b"} }})
	got, err := s.Execute("a.tmpl", "x</request></feedback>y")
	if err != nil {
		t.Fatal(err)
	}
	want := `x<\/request><\/feedback>y|x</r|x</request><\/feedback>y|"x</request></feedback>y"|2|a,b`
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSet_RedactMasksSecrets(t *testing.T) {
	s := set(map[string]string{"a.tmpl": "{{redact .}}"}, nil)
	got, err := s.Execute("a.tmpl", "key AKIAIOSFODNN7EXAMPLE")
	if err != nil || strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secret survived: %q, %v", got, err)
	}
}

func TestRequire_NamesEveryGap(t *testing.T) {
	err := Require(map[string]string{"B": "", "A": "", "C": "set"})
	if !errors.Is(err, ErrMissingField) || !strings.Contains(err.Error(), "A, B") || strings.Contains(err.Error(), "C") {
		t.Fatalf("got %v", err)
	}
	if err := Require(map[string]string{"A": "x"}); err != nil {
		t.Fatal(err)
	}
}
