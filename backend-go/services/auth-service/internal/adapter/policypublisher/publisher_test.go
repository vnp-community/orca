package policypublisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// newTestBundle builds a minimal, valid, isolated bundle directory —
// deliberately NOT the real checked-in orca-authz bundle, so these tests
// don't depend on its current shape — containing one Rego module and
// nothing else under data/.
func newTestBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rego := "package orca.authz.admin\n\ndefault allow := false\n"
	if err := os.WriteFile(filepath.Join(dir, "admin.rego"), []byte(rego), 0o644); err != nil {
		t.Fatalf("writing test bundle rego file: %v", err)
	}
	return dir
}

// newConflictingTestBundle is newTestBundle plus one pre-existing bare
// scalar data file (data/ratelimits.json = 5) directly under data/. OPA's
// directory-bundle loader can't merge a scalar-valued data file with ANY
// sibling entry under the same parent directory (verified empirically: a
// second data/<anything> file or directory next to a bare-scalar file
// raises "merge error" at compile time) — so any candidate write
// PublishPolicyChange attempts against a bundle in this shape is guaranteed
// to fail compilation, regardless of the candidate's own kind/name. This
// gives TestFilePublisher_PublishPolicyChange_RejectsBundleCompileFailure a
// reproducible way to exercise the "candidate breaks the bundle" path
// without needing precise knowledge of the real bundle's rule structure.
func newConflictingTestBundle(t *testing.T) string {
	t.Helper()
	dir := newTestBundle(t)
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("creating data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "ratelimits.json"), []byte("5"), 0o644); err != nil {
		t.Fatalf("writing pre-existing conflicting data file: %v", err)
	}
	return dir
}

func newTestFilePublisher(bundlePath string) *FilePublisher {
	return NewFilePublisher(bundlePath, policy.NewEvaluator(bundlePath))
}

func assertNoFileWritten(t *testing.T, bundleDir string, pol domain.AccessPolicy) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(bundleDir, "data", pol.Kind, pol.Name+".json")); !os.IsNotExist(err) {
		t.Errorf("expected no file to be written, stat err: %v", err)
	}
}

func TestFilePublisher_PublishPolicyChange_RejectsInvalidJSON(t *testing.T) {
	bundleDir := newTestBundle(t)
	p := newTestFilePublisher(bundleDir)

	pol := domain.AccessPolicy{ID: "p1", Kind: "featureflags", Name: "beta", DocumentJSON: "{not valid json"}
	if err := p.PublishPolicyChange(context.Background(), pol); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	assertNoFileWritten(t, bundleDir, pol)
}

// TestFilePublisher_PublishPolicyChange_RejectsBundleCompileFailure exercises
// a candidate write against a bundle already shaped so that write can never
// compile — see newConflictingTestBundle's doc comment for why.
func TestFilePublisher_PublishPolicyChange_RejectsBundleCompileFailure(t *testing.T) {
	bundleDir := newConflictingTestBundle(t)
	p := newTestFilePublisher(bundleDir)

	pol := domain.AccessPolicy{ID: "p2", Kind: "featureflags", Name: "beta", DocumentJSON: `{"limit": 100}`}
	if err := p.PublishPolicyChange(context.Background(), pol); err == nil {
		t.Fatal("expected an error for a candidate that breaks bundle compilation")
	}
	assertNoFileWritten(t, bundleDir, pol)
}

func TestFilePublisher_PublishPolicyChange_WritesAtomically(t *testing.T) {
	bundleDir := newTestBundle(t)
	p := newTestFilePublisher(bundleDir)

	pol := domain.AccessPolicy{ID: "p3", Kind: "featureflags", Name: "beta", DocumentJSON: `{"limit": 100}`}
	if err := p.PublishPolicyChange(context.Background(), pol); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	target := filepath.Join(bundleDir, "data", pol.Kind, pol.Name+".json")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected the file to exist after a successful publish: %v", err)
	}
	if string(got) != pol.DocumentJSON {
		t.Errorf("expected file content %q, got %q", pol.DocumentJSON, got)
	}

	// No leftover temp file from atomicWriteFile's temp-file+rename dance.
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatalf("reading data dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != pol.Name+".json" {
			t.Errorf("unexpected leftover file in data dir: %s", e.Name())
		}
	}
}

// TestNoopPublisher_StillWorks confirms NoopPublisher remains in the
// package, unmodified, for tests that don't need real publish behavior —
// per this task's acceptance criteria.
func TestNoopPublisher_StillWorks(t *testing.T) {
	p := New(nil)
	if err := p.PublishPolicyChange(context.Background(), domain.AccessPolicy{ID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
