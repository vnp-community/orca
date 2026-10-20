package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// PlanRevisionTextKept is how many of the newest versions of a Plan keep the full text of their files. Older versions keep
// their index (paths, sizes, hashes) and digests only, so history stays listable while the table stays bounded.
const PlanRevisionTextKept = 10

// RevisionFile is one entry of a version's file index.
type RevisionFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Omitted bool   `json:"omitted"`
	SHA256  string `json:"sha256"`
}

// OpenSpecChangeRevision is the state of the authored change right after one Plan run. TextKept is false once the version
// fell out of the text window; Bundle is filled only when the version is read on its own, never in a listing.
type OpenSpecChangeRevision struct {
	ID            string
	RequestID     string
	Revision      int
	RunID         string
	Commit        string
	TasksMDDigest string
	BundleDigest  string
	Files         []RevisionFile
	TextKept      bool
	Bundle        map[string]string
	CreatedAt     time.Time
}

// NewRevisionFiles builds the sorted file index of a stored (already masked) bundle. A file kept as an omitted marker takes
// its size and hash from the marker, because the marker is not the file.
func NewRevisionFiles(bundle map[string]string) []RevisionFile {
	paths := make([]string, 0, len(bundle))
	for p := range bundle {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]RevisionFile, 0, len(paths))
	for _, p := range paths {
		content := bundle[p]
		if size, sum, ok := ParseOmittedMarker(content); ok {
			files = append(files, RevisionFile{Path: p, Size: size, Omitted: true, SHA256: sum})
			continue
		}
		sum := sha256.Sum256([]byte(content))
		files = append(files, RevisionFile{Path: p, Size: int64(len(content)), Omitted: isOmittedText(content), SHA256: hex.EncodeToString(sum[:])})
	}
	return files
}

// isOmittedText is true for text that claims to be an omitted marker but does not parse as one; it is listed as omitted so
// a reader never mistakes it for file content.
func isOmittedText(s string) bool { return strings.HasPrefix(s, "<omitted size=") }

type FileChangeKind string

const (
	FileAdded     FileChangeKind = "added"
	FileChanged   FileChangeKind = "changed"
	FileRemoved   FileChangeKind = "removed"
	FileUnchanged FileChangeKind = "unchanged"
)

// FileChange is how one path differs between two versions. Size is the size in the later version, or the earlier one for a
// removed file.
type FileChange struct {
	Path   string
	Kind   FileChangeKind
	Size   int64
	SHA256 string
}

// DiffFileIndex compares two file indexes by path and content hash. A rename is a removal plus an addition: the index holds
// no history of a path. The result is sorted by path.
func DiffFileIndex(prev, cur []RevisionFile) []FileChange {
	before := make(map[string]RevisionFile, len(prev))
	for _, f := range prev {
		before[f.Path] = f
	}
	out := make([]FileChange, 0, len(cur)+len(prev))
	seen := make(map[string]bool, len(cur))
	for _, f := range cur {
		seen[f.Path] = true
		old, existed := before[f.Path]
		switch {
		case !existed:
			out = append(out, FileChange{Path: f.Path, Kind: FileAdded, Size: f.Size, SHA256: f.SHA256})
		case old.SHA256 != f.SHA256 || old.Size != f.Size:
			out = append(out, FileChange{Path: f.Path, Kind: FileChanged, Size: f.Size, SHA256: f.SHA256})
		default:
			out = append(out, FileChange{Path: f.Path, Kind: FileUnchanged, Size: f.Size, SHA256: f.SHA256})
		}
	}
	for _, f := range prev {
		if !seen[f.Path] {
			out = append(out, FileChange{Path: f.Path, Kind: FileRemoved, Size: f.Size, SHA256: f.SHA256})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
