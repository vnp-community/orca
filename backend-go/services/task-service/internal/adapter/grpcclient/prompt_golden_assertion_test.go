package grpcclient

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// updatePromptGolden rewrites the golden prompt files. Use it only for an intentional prompt change, and commit the new goldens in the same change.
var updatePromptGolden = flag.Bool("update", false, "rewrite the golden prompt files under testdata/prompt-golden")

// checkPromptGoldens compares each rendered prompt byte for byte with testdata/prompt-golden/<function>/<case>.txt.
// A golden file with no case left is reported too, so a removed case cannot leave a stale file behind.
func checkPromptGoldens(t *testing.T, function string, rendered map[string]string) {
	t.Helper()
	dir := filepath.Join("testdata", "prompt-golden", function)
	names := make([]string, 0, len(rendered))
	for name := range rendered {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			comparePromptGolden(t, filepath.Join(dir, name+".txt"), rendered[name])
		})
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !*updatePromptGolden {
		t.Fatalf("golden directory %s unreadable: %v", dir, err)
	}
	for _, e := range entries {
		if _, ok := rendered[strings.TrimSuffix(e.Name(), ".txt")]; ok && strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		if *updatePromptGolden {
			if rmErr := os.Remove(filepath.Join(dir, e.Name())); rmErr != nil {
				t.Errorf("remove stale golden %s: %v", e.Name(), rmErr)
			}
			continue
		}
		t.Errorf("golden file %s/%s has no test case; delete it or restore the case", dir, e.Name())
	}
}

func comparePromptGolden(t *testing.T, path, got string) {
	t.Helper()
	if *updatePromptGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (create it deliberately with: go test -run TestPromptGolden -update): %v", path, err)
	}
	if string(want) == got {
		return
	}
	t.Fatalf("prompt differs from %s (golden %d bytes, rendered %d bytes); %s\nIf the change is intended, regenerate with -update and commit the golden.", path, len(want), len(got), firstPromptDifference(string(want), got))
}

// firstPromptDifference names the first differing line so a failure stays readable for prompts of tens of kilobytes.
func firstPromptDifference(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl || i >= len(w) || i >= len(g) {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n  golden:   " + clipLine(wl) + "\n  rendered: " + clipLine(gl)
		}
	}
	return "no line differs (trailing bytes differ)"
}

func clipLine(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
