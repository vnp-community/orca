// Package render parses the embedded .tmpl files of one prompt flow and renders them with the shared template functions.
package render

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/stablyai/orca-go/prompts/fence"
	"github.com/stablyai/orca-go/prompts/redact"
)

// ErrMissingField is returned when an Input lacks a field its prompt cannot be written without.
var ErrMissingField = errors.New("prompt input is missing a required field")

// Funcs are the functions every prompt template may call. User-written text goes through fence; nothing else escapes it.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"fence":    fence.Untrusted,
		"closing":  fence.Closing,
		"truncate": fence.Truncate,
		"redact":   redact.Secrets,
		"join":     strings.Join,
		"quote":    strconv.Quote,
		"inc":      func(i int) int { return i + 1 },
	}
}

// Set is the parsed templates of one flow. It parses on first use so a package import costs nothing.
type Set struct {
	funcs template.FuncMap
	files []fs.FS
	once  sync.Once
	tmpl  *template.Template
	err   error
}

// NewSet takes the flow's own files and any partial packages it includes. File names must be unique across all of them.
func NewSet(funcs template.FuncMap, files ...fs.FS) *Set {
	return &Set{funcs: funcs, files: files}
}

func (s *Set) load() (*template.Template, error) {
	s.once.Do(func() {
		t := template.New("prompt").Option("missingkey=error").Funcs(Funcs()).Funcs(s.funcs)
		for _, f := range s.files {
			if _, err := t.ParseFS(f, "*.tmpl"); err != nil {
				s.err = fmt.Errorf("parse prompt templates: %w", err)
				return
			}
		}
		s.tmpl = t
	})
	return s.tmpl, s.err
}

// Execute renders the named template file with data.
func (s *Set) Execute(name string, data any) (string, error) {
	t, err := s.load()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("render %s: %w", name, err)
	}
	return b.String(), nil
}

// Require returns ErrMissingField naming every empty value, so a caller learns all gaps at once.
func Require(fields map[string]string) error {
	var missing []string
	for name, v := range fields {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("%w: %s", ErrMissingField, strings.Join(missing, ", "))
}
