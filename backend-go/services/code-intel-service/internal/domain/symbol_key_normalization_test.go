package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type vectorTestCase struct {
	Name            string `json:"name"`
	Tool            string `json:"tool"`
	Input           struct {
		Kind          string `json:"kind"`
		FilePath      string `json:"filePath"`
		QualifiedName string `json:"qualifiedName"`
		Name          string `json:"name"`
		LineBase      int    `json:"lineBase"`
		StartLine     int    `json:"startLine"`
	} `json:"input"`
	ExpectKey       string `json:"expectKey"`
	ExpectStartLine int    `json:"expectStartLine"`
	ExpectError     string `json:"expectError"`
}

func TestSymbolVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "symbol-vectors", "symbol-key-vectors.json"))
	if err != nil {
		t.Fatalf("failed to read symbol vectors: %v", err)
	}

	var cases []vectorTestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to parse vectors: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ExpectError != "" {
				_, err := NormalizeRepoPath(tc.Input.FilePath, "/home/u/repo", PlatformLinux)
				if err == nil {
					t.Fatalf("expected error containing %s, got nil", tc.ExpectError)
				}
				return
			}

			var ref SymbolRef
			if tc.Tool == "gitnexus" {
				ref = SymbolRefFromGitNexus(GitNexusNode{
					Label:         tc.Input.Kind,
					FilePath:      tc.Input.FilePath,
					QualifiedName: tc.Input.QualifiedName,
					Name:          tc.Input.Name,
					LineBase:      tc.Input.LineBase,
					StartLine:     tc.Input.StartLine,
				}, tc.Input.LineBase)
			} else {
				ref = SymbolRefFromCodeGraph(CodeGraphNode{
					Kind:          tc.Input.Kind,
					FilePath:      tc.Input.FilePath,
					QualifiedName: tc.Input.QualifiedName,
					Name:          tc.Input.Name,
					StartLine:     tc.Input.StartLine,
				})
			}

			if strings.Contains(tc.Name, "collision") {
				refs := ResolveKeyCollisions([]SymbolRef{ref, ref})
				ref = refs[0]
			}

			if tc.ExpectKey != "" && ref.Key != tc.ExpectKey {
				t.Errorf("key mismatch:\ngot:  %s\nwant: %s", ref.Key, tc.ExpectKey)
			}
			if tc.ExpectStartLine != 0 && int(ref.StartLine) != tc.ExpectStartLine {
				t.Errorf("startLine mismatch: got %d, want %d", ref.StartLine, tc.ExpectStartLine)
			}
		})
	}
}

func TestResolveKeyCollisions(t *testing.T) {
	refs := []SymbolRef{
		{
			Key:       "method:src/calc.ts:Calc.add",
			StartLine: 10,
			Ordinal:   1,
		},
		{
			Key:       "method:src/calc.ts:Calc.add",
			StartLine: 25,
			Ordinal:   2,
		},
		{
			Key:       "function:src/calc.ts:sub",
			StartLine: 40,
		},
	}

	resolved := ResolveKeyCollisions(refs)
	if resolved[0].Key != "method:src/calc.ts:Calc.add#1" {
		t.Errorf("expected #1 suffix, got %s", resolved[0].Key)
	}
	if resolved[1].Key != "method:src/calc.ts:Calc.add#2" {
		t.Errorf("expected #2 suffix, got %s", resolved[1].Key)
	}
	if resolved[2].Key != "function:src/calc.ts:sub" {
		t.Errorf("uncollided symbol should not have suffix: got %s", resolved[2].Key)
	}
}

func TestValidateAgentSymbolRef(t *testing.T) {
	ref := SymbolRef{
		Key:           "wrong_key",
		Kind:          SymbolKindFunction,
		Name:          "doWork",
		QualifiedName: "doWork",
		FilePath:      "src/worker.ts",
	}

	validated, mismatch, err := ValidateAgentSymbolRef(ref, "", PlatformLinux)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mismatch {
		t.Fatal("expected mismatch flag to be true")
	}
	expected := "function:src/worker.ts:doWork"
	if validated.Key != expected {
		t.Fatalf("expected key %s, got %s", expected, validated.Key)
	}
}

func TestEdgeKindMapping(t *testing.T) {
	if k, ok := EdgeKindFromGitNexus("CALLS"); !ok || k != EdgeKindCalls {
		t.Fatalf("expected EdgeKindCalls, got %v, %v", k, ok)
	}
	if _, ok := EdgeKindFromGitNexus("DEFINES"); ok {
		t.Fatal("expected structural edge DEFINES to return false")
	}
}
