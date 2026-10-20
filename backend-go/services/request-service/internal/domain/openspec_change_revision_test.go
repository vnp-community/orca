package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func sumOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestNewRevisionFiles_OmittedMarkerSizeAndHash(t *testing.T) {
	hash := strings.Repeat("c", 64)
	files := NewRevisionFiles(map[string]string{
		"tasks.md":    "# t\n",
		"big.md":      fmt.Sprintf("<omitted size=%d sha256=%s>", 7340032, hash),
		"specs/a.md":  "x",
		"broken.md":   "<omitted size=oops>",
		"empty.md":    "",
		"specs/b.md":  "yy",
		"design.md":   "d",
		"proposal.md": "p",
	})
	var paths []string
	byPath := map[string]RevisionFile{}
	for _, f := range files {
		paths = append(paths, f.Path)
		byPath[f.Path] = f
	}
	for i := 1; i < len(paths); i++ {
		if paths[i-1] >= paths[i] {
			t.Fatalf("the index must be sorted by path: %v", paths)
		}
	}
	if f := byPath["big.md"]; !f.Omitted || f.Size != 7340032 || f.SHA256 != hash {
		t.Fatalf("an omitted file takes size and hash from its marker: %+v", f)
	}
	if f := byPath["tasks.md"]; f.Omitted || f.Size != 4 || f.SHA256 != sumOf("# t\n") {
		t.Fatalf("%+v", f)
	}
	if f := byPath["broken.md"]; !f.Omitted {
		t.Fatalf("text that claims to be a marker is never listed as content: %+v", f)
	}
	if f := byPath["empty.md"]; f.Omitted || f.Size != 0 || f.SHA256 != sumOf("") {
		t.Fatalf("%+v", f)
	}
}

func idx(pairs ...string) []RevisionFile {
	var out []RevisionFile
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, RevisionFile{Path: pairs[i], Size: int64(len(pairs[i+1])), SHA256: sumOf(pairs[i+1])})
	}
	return out
}

func kinds(changes []FileChange) map[string]FileChangeKind {
	m := map[string]FileChangeKind{}
	for _, c := range changes {
		m[c.Path] = c.Kind
	}
	return m
}

func TestDiffFileIndex_AddedChangedRemovedUnchanged(t *testing.T) {
	prev := idx("tasks.md", "one", "design.md", "same", "specs/old/spec.md", "gone")
	cur := idx("tasks.md", "two", "design.md", "same", "specs/new/spec.md", "fresh")
	got := kinds(DiffFileIndex(prev, cur))
	want := map[string]FileChangeKind{"tasks.md": FileChanged, "design.md": FileUnchanged, "specs/old/spec.md": FileRemoved, "specs/new/spec.md": FileAdded}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for p, k := range want {
		if got[p] != k {
			t.Errorf("%s = %s, want %s", p, got[p], k)
		}
	}
}

func TestDiffFileIndex_RenameIsRemoveAndAdd(t *testing.T) {
	got := kinds(DiffFileIndex(idx("a.md", "same text"), idx("b.md", "same text")))
	if got["a.md"] != FileRemoved || got["b.md"] != FileAdded {
		t.Fatalf("%v", got)
	}
}

func TestDiffFileIndex_RemovedFileCarriesThePreviousSize(t *testing.T) {
	for _, c := range DiffFileIndex(idx("a.md", "12345"), nil) {
		if c.Kind != FileRemoved || c.Size != 5 {
			t.Fatalf("%+v", c)
		}
	}
}

func TestDiffFileIndex_EmptyAndSorted(t *testing.T) {
	if got := DiffFileIndex(nil, nil); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	got := DiffFileIndex(idx("z.md", "1"), idx("b.md", "1", "a.md", "1"))
	if len(got) != 3 || got[0].Path != "a.md" || got[1].Path != "b.md" || got[2].Path != "z.md" {
		t.Fatalf("%v", got)
	}
}
