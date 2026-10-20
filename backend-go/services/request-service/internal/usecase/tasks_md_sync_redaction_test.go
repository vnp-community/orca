package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// maskedSyncWorld stores the change as the proposal run does when it masked a secret: the bundle holds the mask, the
// digest is of the raw file the agent wrote.
func maskedSyncWorld(t *testing.T) *syncWorld {
	t.Helper()
	w := newSyncWorld(t)
	masked := strings.Replace(syncTasksMd, "Done when: one", "Done when: [REDACTED]", 1)
	w.change.Bundle = map[string]string{"tasks.md": masked}
	w.change.RedactedSecrets = 1
	w.change.TasksMDDigest = domain.PlanRegionDigest(syncTasksMd)
	if _, err := w.changes.Upsert(context.Background(), w.change); err != nil {
		t.Fatal(err)
	}
	w.build()
	return w
}

func TestRestoreDrift_RedactedBundle_NeverWritesMask(t *testing.T) {
	w := maskedSyncWorld(t)
	w.ws.files[syncTasksPath] = []byte(strings.Replace(syncTasksMd, "- [ ] T1.2 two [type=feature] [h=1] [depends=T1.1]", "- [x] T1.2 two [type=feature] [h=9]", 1))
	w.done["task-1"] = true
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	md := w.tasksMd()
	if strings.Contains(md, "[REDACTED]") {
		t.Fatalf("the mask reached the real tasks.md:\n%s", md)
	}
	if !strings.Contains(md, "- [x] T1.2 two [type=feature] [h=9]") || !strings.Contains(md, "- [x] T1.1 one") {
		t.Fatalf("the hand edit stays and the done task is ticked:\n%s", md)
	}
	if w.rec.drift != 1 {
		t.Fatalf("the drift must still be counted once: %d", w.rec.drift)
	}
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil || w.rec.drift != 1 {
		t.Fatalf("a second pass must not warn again: drift=%d err=%v", w.rec.drift, err)
	}
}

func TestRestoreDrift_RedactedBundle_UntouchedFileIsNotDrift(t *testing.T) {
	w := maskedSyncWorld(t)
	w.done["task-1"] = true
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	if w.rec.drift != 0 || strings.Contains(w.tasksMd(), "[REDACTED]") || !strings.Contains(w.tasksMd(), "- [x] T1.1 one") {
		t.Fatalf("drift=%d\n%s", w.rec.drift, w.tasksMd())
	}
}

func TestRestoreDrift_RedactedBundle_BaselineAfterSyncIsTheWrittenRegion(t *testing.T) {
	c := domain.OpenSpecChange{RedactedSecrets: 2, Bundle: map[string]string{"tasks.md": "[REDACTED]"}}
	region, _ := domain.PlanRegionText(syncTasksMd)
	c.SyncDigest = domain.PlanRegionDigest(region)
	s := &TasksMdSyncer{}
	if got, drift, err := s.restoreDrift(c, syncTasksMd, region); err != nil || drift != driftNone || got != syncTasksMd {
		t.Fatalf("drift=%v err=%v", drift, err)
	}
	if got, drift, err := s.restoreDrift(c, syncTasksMd, region+"\n- [ ] T1.3 x"); err != nil || drift != driftKept || got != syncTasksMd {
		t.Fatalf("an edited region is kept as it is: drift=%v err=%v", drift, err)
	}
}

func TestRestoreDrift_NoRedaction_UnchangedBehaviour(t *testing.T) {
	region, _ := domain.PlanRegionText(syncTasksMd)
	c := domain.OpenSpecChange{Bundle: map[string]string{"tasks.md": syncTasksMd}}
	s := &TasksMdSyncer{}
	if _, drift, err := s.restoreDrift(c, syncTasksMd, region); err != nil || drift != driftNone {
		t.Fatalf("drift=%v err=%v", drift, err)
	}
	edited := strings.Replace(syncTasksMd, "two", "TWO", 1)
	editedRegion, _ := domain.PlanRegionText(edited)
	got, drift, err := s.restoreDrift(c, edited, editedRegion)
	if err != nil || drift != driftRestored || got != syncTasksMd {
		t.Fatalf("the stored region must be put back: drift=%v err=%v\n%s", drift, err, got)
	}
}
