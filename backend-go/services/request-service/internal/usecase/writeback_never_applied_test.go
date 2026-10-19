package usecase

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Orca only stores and shows what an agent proposes for the design knowledge; a person applies it (decision T1.7). This
// reads the source of Orca's Go modules and of the agent and fails if any of it could change the knowledge graph.
func TestNoSourceOfOrcaOrItsAgentCanApplyAChangeToTheKnowledgeGraph(t *testing.T) {
	forbidden := []string{"kg apply", "kg import", "kg_apply", "changeset_apply", "apply_changeset", "mcp__tdd-agent__changeset_apply"}
	roots := []string{"../../../../", "../../../../../agent/src/"}
	scanned := 0
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "node_modules", "gen", ".git", "testdata", "out", "__fixtures__":
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".ts")) || strings.HasSuffix(name, "_test.go") ||
				strings.HasSuffix(name, ".test.ts") || name == "writeback_never_applied_test.go" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			text := strings.ToLower(string(raw))
			for _, word := range forbidden {
				if strings.Contains(text, word) {
					t.Errorf("%s mentions %q: Orca must not be able to change the knowledge graph", path, word)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 200 {
		t.Fatalf("only %d source files were scanned; the roots are wrong", scanned)
	}
}
