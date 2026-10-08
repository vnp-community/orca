package domain

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// readmeParagraphAfter returns the first non-empty paragraph after the heading that starts with prefix.
func readmeParagraphAfter(t *testing.T, readme, prefix string) string {
	t.Helper()
	lines := strings.Split(readme, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, prefix) {
			continue
		}
		var para []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				if len(para) > 0 {
					break
				}
				continue
			}
			para = append(para, next)
		}
		return strings.Join(para, " ")
	}
	t.Fatalf("heading %q not found in README", prefix)
	return ""
}

func backticked(s string) []string {
	var out []string
	for _, m := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestREADMEListsSameTypesAndStatuses(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "docs", "crs", "v6", "README.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("v6 README not reachable from this checkout: %v", err)
	}
	readme := string(b)

	types := backticked(readmeParagraphAfter(t, readme, "### 3.2"))
	var codeTypes []string
	for _, rt := range AllRequestTypes() {
		codeTypes = append(codeTypes, string(rt))
	}
	sort.Strings(types)
	sort.Strings(codeTypes)
	if !reflect.DeepEqual(types, codeTypes) {
		t.Errorf("README types %v != code types %v", types, codeTypes)
	}

	statuses := backticked(readmeParagraphAfter(t, readme, "### 3.3"))
	var codeStatuses []string
	for _, s := range AllRequestStatuses() {
		codeStatuses = append(codeStatuses, string(s))
	}
	sort.Strings(statuses)
	sort.Strings(codeStatuses)
	if !reflect.DeepEqual(statuses, codeStatuses) {
		t.Errorf("README statuses %v != code statuses %v", statuses, codeStatuses)
	}
}
