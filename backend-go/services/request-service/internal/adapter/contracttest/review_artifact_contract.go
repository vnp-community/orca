package contracttest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func artifactOf(req string, key domain.ArtifactKey, state domain.ReviewArtifactState, content string) (domain.ReviewArtifact, *domain.ReviewBlob) {
	a := domain.ReviewArtifact{
		ID: uuid.NewString(), RequestID: req, Key: key, Type: domain.DiagramArchitecture, State: state, Origin: domain.ArtifactOriginAuto,
		ViewBox: "0 0 960 540", Title: "Containers", Desc: "A drawing", Model: []byte(`{"section":"architecture"}`),
	}
	if state == domain.ArtifactFailed {
		a.ErrorCode = "REQUEST_REVIEW_ARTIFACT_UNSAFE"
		return a, nil
	}
	blob := &domain.ReviewBlob{Digest: domain.BlobDigest([]byte(content)), MediaType: domain.ReviewArtifactMediaType, Content: []byte(content)}
	a.BlobDigest, a.SizeBytes = blob.Digest, len(content)
	return a, blob
}

func keyOf(input string) domain.ArtifactKey {
	return domain.ArtifactKey{Section: domain.SectionArchitecture, Variant: domain.DiagramVariantDelta, InputDigest: input, PresetDigest: "preset-1", SkillVersion: "skill-1"}
}

// RunReviewArtifactContract is what both dialects must do for stored drawings: revisions of one key, one active drawing per
// slot even when writers race, content kept once per Request and digest, restore without a new row, and no reads across tenants.
func RunReviewArtifactContract(t *testing.T, repo usecase.ReviewArtifactRepository) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	req := uuid.NewString()
	save := func(ctx context.Context, key domain.ArtifactKey, state domain.ReviewArtifactState, content string) domain.ReviewArtifact {
		t.Helper()
		a, blob := artifactOf(req, key, state, content)
		got, err := repo.Save(ctx, a, blob)
		if err != nil {
			t.Fatalf("save %s: %v", key.InputDigest, err)
		}
		return got
	}

	t.Run("save and read back with its content", func(t *testing.T) {
		got := save(ctxA, keyOf("m1"), domain.ArtifactActive, "<svg>one</svg>")
		if got.TenantID != tenantA || got.Revision != 1 || got.State != domain.ArtifactActive || got.ViewBox != "0 0 960 540" || got.CreatedAt.Location() != time.UTC {
			t.Fatalf("saved %+v", got)
		}
		if !jsonEqual(got.Model, []byte(`{"section":"architecture"}`)) {
			t.Errorf("model %s", got.Model)
		}
		act, ok, err := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		if err != nil || !ok || act.ID != got.ID {
			t.Fatalf("active: %+v %v %v", act, ok, err)
		}
		blob, ok, err := repo.Blob(ctxA, req, got.BlobDigest)
		if err != nil || !ok || string(blob.Content) != "<svg>one</svg>" || domain.VerifyBlob(blob) != nil {
			t.Fatalf("blob: %+v %v %v", blob, ok, err)
		}
		byKey, ok, _ := repo.FindByKey(ctxA, req, keyOf("m1"))
		if !ok || byKey.ID != got.ID {
			t.Errorf("by key: %+v", byKey)
		}
		if _, ok, _ := repo.FindByKey(ctxA, req, keyOf("never")); ok {
			t.Error("an unknown key finds nothing")
		}
		if _, err := repo.Get(ctxA, req, got.ID); err != nil {
			t.Errorf("get: %v", err)
		}
		if _, err := repo.Get(ctxA, uuid.NewString(), got.ID); appCode(err) != "REQUEST_REVIEW_ARTIFACT_NOT_FOUND" {
			t.Errorf("a drawing is read through its own Request: %v", err)
		}
	})

	t.Run("the same key gets the next revision and the content is kept once", func(t *testing.T) {
		second := save(ctxA, keyOf("m1"), domain.ArtifactActive, "<svg>one</svg>")
		third := save(ctxA, keyOf("m1"), domain.ArtifactActive, "<svg>one again</svg>")
		if second.Revision != 2 || third.Revision != 3 {
			t.Fatalf("revisions %d %d", second.Revision, third.Revision)
		}
		revs, err := repo.ListRevisions(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		if err != nil || len(revs) < 3 || revs[0].ID != third.ID {
			t.Fatalf("newest first: %+v %v", revs, err)
		}
		active := 0
		for _, r := range revs {
			if r.State == domain.ArtifactActive {
				active++
			}
		}
		if active != 1 {
			t.Errorf("%d active drawings in one slot", active)
		}
		n, err := repo.StoredBytes(ctxA)
		want := int64(len("<svg>one</svg>") + len("<svg>one again</svg>")) // the first content was saved twice and stored once
		if err != nil || n != want {
			t.Errorf("stored bytes = %d, want %d (%v)", n, want, err)
		}
	})

	t.Run("a new model takes the slot and an old drawing can be restored for free", func(t *testing.T) {
		old, _, _ := repo.FindByKey(ctxA, req, keyOf("m1"))
		next := save(ctxA, keyOf("m2"), domain.ArtifactActive, "<svg>two</svg>")
		act, _, _ := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		if act.ID != next.ID {
			t.Fatalf("the new drawing must be active: %+v", act)
		}
		prev, err := repo.Get(ctxA, req, old.ID)
		if err != nil || prev.State != domain.ArtifactSuperseded {
			t.Fatalf("the old one is superseded: %+v %v", prev, err)
		}
		restored, err := repo.Activate(ctxA, req, old.ID)
		if err != nil || restored.State != domain.ArtifactActive || restored.ID != old.ID {
			t.Fatalf("restore: %+v %v", restored, err)
		}
		if act, _, _ = repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, ""); act.ID != old.ID {
			t.Errorf("active after restore: %+v", act)
		}
		if again, _ := repo.Get(ctxA, req, next.ID); again.State != domain.ArtifactSuperseded {
			t.Errorf("the one it replaced is superseded: %+v", again)
		}
		if _, err := repo.Activate(ctxA, req, uuid.NewString()); appCode(err) != "REQUEST_REVIEW_ARTIFACT_NOT_FOUND" {
			t.Errorf("restore of nothing: %v", err)
		}
	})

	t.Run("a failed attempt is kept, has no content and does not take the slot", func(t *testing.T) {
		before, _, _ := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		failed := save(ctxA, keyOf("m3"), domain.ArtifactFailed, "")
		if failed.State != domain.ArtifactFailed || failed.BlobDigest != "" || failed.ErrorCode == "" {
			t.Fatalf("failed %+v", failed)
		}
		if act, _, _ := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, ""); act.ID != before.ID {
			t.Errorf("a failure must not replace the drawing in force")
		}
		if found, ok, _ := repo.FindByKey(ctxA, req, keyOf("m3")); !ok || found.State != domain.ArtifactFailed {
			t.Errorf("the failure is found by its key so it is not retried by itself: %+v", found)
		}
		if _, err := repo.Activate(ctxA, req, failed.ID); err == nil {
			t.Error("a failed drawing cannot be restored")
		}
	})

	t.Run("slots are separate by section, variant and name", func(t *testing.T) {
		flow := keyOf("f1")
		flow.Section, flow.Name = domain.SectionFlow, "Bug status"
		other := flow
		other.Name = "Notifications"
		a := save(ctxA, flow, domain.ArtifactActive, "<svg>flow a</svg>")
		b := save(ctxA, other, domain.ArtifactActive, "<svg>flow b</svg>")
		if x, _, _ := repo.Active(ctxA, req, domain.SectionFlow, domain.DiagramVariantDelta, "Bug status"); x.ID != a.ID {
			t.Errorf("flow a: %+v", x)
		}
		if x, _, _ := repo.Active(ctxA, req, domain.SectionFlow, domain.DiagramVariantDelta, "Notifications"); x.ID != b.ID {
			t.Errorf("flow b: %+v", x)
		}
		act, err := repo.ListActive(ctxA, req)
		if err != nil || len(act) != 3 {
			t.Errorf("active drawings: %d %v", len(act), err)
		}
	})

	t.Run("racing writers leave exactly one active drawing and no error", func(t *testing.T) {
		race := uuid.NewString()
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				a, blob := artifactOf(race, keyOf(fmt.Sprintf("race-%d", i)), domain.ArtifactActive, fmt.Sprintf("<svg>%d</svg>", i))
				if _, err := repo.Save(ctxA, a, blob); err != nil {
					errs <- err
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Errorf("a writer failed: %v", err)
		}
		act, err := repo.ListActive(ctxA, race)
		if err != nil || len(act) != 1 {
			t.Fatalf("active = %d (%v)", len(act), err)
		}
		revs, _ := repo.ListRevisions(ctxA, race, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		if len(revs) != 8 {
			t.Errorf("every attempt is kept: %d", len(revs))
		}
	})

	t.Run("the same key saved at once gets distinct revisions", func(t *testing.T) {
		same := uuid.NewString()
		var wg sync.WaitGroup
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				a, blob := artifactOf(same, keyOf("same"), domain.ArtifactActive, "<svg>same</svg>")
				if _, err := repo.Save(ctxA, a, blob); err != nil {
					t.Errorf("writer: %v", err)
				}
			}()
		}
		wg.Wait()
		revs, _ := repo.ListRevisions(ctxA, same, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		seen := map[int]bool{}
		for _, r := range revs {
			if seen[r.Revision] {
				t.Errorf("revision %d twice", r.Revision)
			}
			seen[r.Revision] = true
		}
		if len(revs) != 6 {
			t.Errorf("revisions: %d", len(revs))
		}
	})

	t.Run("tenants do not see each other", func(t *testing.T) {
		if _, ok, _ := repo.Active(ctxB, req, domain.SectionArchitecture, domain.DiagramVariantDelta, ""); ok {
			t.Error("tenant B saw tenant A's drawing")
		}
		a, _, _ := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, "")
		if _, ok, _ := repo.Blob(ctxB, req, a.BlobDigest); ok {
			t.Error("tenant B read tenant A's content")
		}
		if _, err := repo.Get(ctxB, req, a.ID); appCode(err) != "REQUEST_REVIEW_ARTIFACT_NOT_FOUND" {
			t.Errorf("get across tenants: %v", err)
		}
		if _, err := repo.Activate(ctxB, req, a.ID); err == nil {
			t.Error("tenant B restored tenant A's drawing")
		}
		if n, _ := repo.StoredBytes(ctxB); n != 0 {
			t.Errorf("tenant B stores %d bytes", n)
		}
		bArt, bBlob := artifactOf(req, keyOf("m1"), domain.ArtifactActive, "<svg>b</svg>")
		if _, err := repo.Save(ctxB, bArt, bBlob); err != nil {
			t.Fatalf("the same key in another tenant is another row: %v", err)
		}
		if x, _, _ := repo.Active(ctxA, req, domain.SectionArchitecture, domain.DiagramVariantDelta, ""); x.ID != a.ID {
			t.Error("tenant B's save changed tenant A's slot")
		}
	})

	t.Run("cleanup removes old inactive drawings and unreferenced content only", func(t *testing.T) {
		clean := uuid.NewString()
		first, b1 := artifactOf(clean, keyOf("c1"), domain.ArtifactActive, "<svg>c1</svg>")
		if _, err := repo.Save(ctxA, first, b1); err != nil {
			t.Fatal(err)
		}
		second, b2 := artifactOf(clean, keyOf("c2"), domain.ArtifactActive, "<svg>c2</svg>")
		if _, err := repo.Save(ctxA, second, b2); err != nil {
			t.Fatal(err)
		}
		failed, _ := artifactOf(clean, keyOf("c3"), domain.ArtifactFailed, "")
		if _, err := repo.Save(ctxA, failed, nil); err != nil {
			t.Fatal(err)
		}
		future := time.Now().Add(time.Hour)
		if n, err := repo.DeleteSupersededBefore(ctxA, time.Now().Add(-time.Hour), 100); err != nil || n != 0 {
			t.Fatalf("nothing is old yet: %d %v", n, err)
		}
		if n, err := repo.DeleteSupersededBefore(ctxA, future, 100); err != nil {
			t.Fatal(err)
		} else if n < 2 {
			t.Errorf("the superseded and the failed drawings go: %d", n)
		}
		if act, _ := repo.ListActive(ctxA, clean); len(act) != 1 || act[0].Key.InputDigest != "c2" {
			t.Errorf("the active drawing stays: %+v", act)
		}
		if _, err := repo.DeleteOrphanBlobs(ctxA, future, 100); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := repo.Blob(ctxA, clean, b2.Digest); !ok {
			t.Error("content an active drawing points at must stay")
		}
		if _, ok, _ := repo.Blob(ctxA, clean, b1.Digest); ok {
			t.Error("content no drawing points at must go")
		}
	})

	t.Run("the cleanup never removes content a saving drawing is about to use", func(t *testing.T) {
		race := uuid.NewString()
		first, blob := artifactOf(race, keyOf("seed"), domain.ArtifactActive, "<svg>shared</svg>")
		if _, err := repo.Save(ctxA, first, blob); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.DeleteSupersededBefore(ctxA, time.Now().Add(time.Hour), 100); err != nil { // the seed row goes, its content is orphaned
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		stop := make(chan struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = repo.DeleteOrphanBlobs(ctxA, time.Now(), 100)
				}
			}
		}()
		for i := 0; i < 25; i++ {
			a, b := artifactOf(race, keyOf(fmt.Sprintf("again-%d", i)), domain.ArtifactActive, "<svg>shared</svg>")
			if _, err := repo.Save(ctxA, a, b); err != nil {
				t.Fatalf("save %d: %v", i, err)
			}
		}
		close(stop)
		wg.Wait()
		active, err := repo.ListActive(ctxA, race)
		if err != nil || len(active) != 1 {
			t.Fatalf("active: %d %v", len(active), err)
		}
		if blob, ok, _ := repo.Blob(ctxA, race, active[0].BlobDigest); !ok || domain.VerifyBlob(blob) != nil {
			t.Error("the drawing in force lost its content to the cleanup")
		}
	})
}
