package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

type fakeSyncState struct {
	last  map[string]int64
	err   error
	calls int
}

func (f *fakeSyncState) Advance(_ context.Context, tenantID, requestID string, version int64, _ string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if f.last == nil {
		f.last = map[string]int64{}
	}
	key := tenantID + "/" + requestID
	if prev, ok := f.last[key]; ok && prev >= version {
		return false, nil
	}
	f.last[key] = version
	return true, nil
}

type recordingObserver struct {
	results     []string
	owned       []string
	transitions int
}

func (r *recordingObserver) ObserveRequestEvent(event, result string) {
	r.results = append(r.results, event+":"+result)
}
func (r *recordingObserver) ObserveRequestOwnedSkip(source string) { r.owned = append(r.owned, source) }
func (r *recordingObserver) ObserveJiraTransition(time.Duration)   { r.transitions++ }

type fakeCommenter struct {
	bodies []string
	err    error
}

func (f *fakeCommenter) AddComment(_ context.Context, _, _, _, _, _, body string) error {
	f.bodies = append(f.bodies, body)
	return f.err
}

type requestHarness struct {
	*syncHarness
	state *fakeSyncState
	obs   *recordingObserver
}

func newRequestHarness(opts ...Option) *requestHarness {
	h := &requestHarness{syncHarness: &syncHarness{tracker: &fakeTracker{category: "todo"}, scm: &fakeScm{}, projects: &fakeProjects{enabled: true}, processed: newFakeProcessedEvents()},
		state: &fakeSyncState{}, obs: &recordingObserver{}}
	all := append([]Option{WithRequestSyncState(h.state), WithObserver(h.obs)}, opts...)
	h.uc = NewSyncIssueStatus(h.tracker, h.scm, h.projects, h.processed, nil, all...)
	return h
}

func requestEvent(id, typ, to string) domain.RequestStatusEvent {
	return domain.RequestStatusEvent{
		EventID: id, TenantID: "t1", RequestID: "r1", Type: typ, To: to, Version: 1, Number: 42,
		ActorID: "user-7", ActorKind: "user", ReporterID: "user-9",
		SourceProvider: "jira", SourceRef: "ENG-1", SourceSite: "https://a.atlassian.net",
	}
}

var allRequestTypes = []string{"change_request", "bug", "hotfix", "task", "spike", "question", "refactor", "security", "performance", "docs", "ops_request"}

func TestHandleRequestStatus_MappingTableForAllElevenTypes(t *testing.T) {
	for _, typ := range allRequestTypes {
		analysisOnly := typ == "spike" || typ == "question"
		cases := []struct {
			to        string
			completed bool
			want      string // "" means no tracker call
			wantFrom  []string
		}{
			{"executing", false, map[bool]string{true: "", false: "In Progress"}[analysisOnly], []string{"todo"}},
			{"analyzing", false, map[bool]string{true: "In Progress", false: ""}[analysisOnly], []string{"todo"}},
			{"completed", true, "Done", []string{"todo", "in_progress"}},
			{"request_backlog", false, "", nil},
			{"cancelled", false, "", nil},
			{"awaiting_type_confirmation", false, "", nil},
			{"awaiting_plan_approval", false, "", nil},
			{"classifying", false, "", nil},
			{"planning", false, "", nil},
		}
		for _, tc := range cases {
			t.Run(typ+"/"+tc.to, func(t *testing.T) {
				h := newRequestHarness()
				ev := requestEvent("ev-1", typ, tc.to)
				ev.Completed = tc.completed
				if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
					t.Fatal(err)
				}
				if tc.want == "" {
					if h.tracker.calls != 0 || h.tracker.catCalls != 0 {
						t.Fatalf("must not touch the tracker, got transitions=%d category=%d", h.tracker.calls, h.tracker.catCalls)
					}
					return
				}
				if h.tracker.calls != 1 || h.tracker.gotState != tc.want || h.tracker.gotUser != "user-7" ||
					h.tracker.gotRef != "ENG-1" || h.tracker.gotSite != "https://a.atlassian.net" {
					t.Fatalf("want %q on ENG-1 as user-7, got %+v", tc.want, h.tracker)
				}
				got := mapRequestEventToStatus(ev, DefaultStatusNames())
				if !reflect.DeepEqual(got.OnlyFromCategories, tc.wantFrom) {
					t.Errorf("OnlyFromCategories = %v, want %v", got.OnlyFromCategories, tc.wantFrom)
				}
			})
		}
	}
}

func TestHandleRequestStatus_ConfiguredStatusNames(t *testing.T) {
	h := newRequestHarness(WithStatusNames(StatusNames{InProgress: "Doing", Done: "Closed"}))
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.gotState != "Doing" {
		t.Errorf("state = %q, want Doing", h.tracker.gotState)
	}
}

func TestHandleRequestStatus_CategoryGuardSkipsAndCounts(t *testing.T) {
	h := newRequestHarness()
	h.tracker.category = "in_progress" // executing only moves todo issues
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.calls != 0 || !reflect.DeepEqual(h.obs.results, []string{"status_changed:skipped_category"}) {
		t.Errorf("calls=%d results=%v", h.tracker.calls, h.obs.results)
	}
}

func TestHandleRequestStatus_CompletedFromInProgressAndTodoButNotDone(t *testing.T) {
	for category, wantCalls := range map[string]int{"todo": 1, "in_progress": 1, "done": 0} {
		h := newRequestHarness()
		h.tracker.category = category
		ev := requestEvent("ev-1", "spike", "completed")
		ev.Completed = true
		if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
		if h.tracker.calls != wantCalls {
			t.Errorf("category %s: transitions = %d, want %d", category, h.tracker.calls, wantCalls)
		}
	}
}

func TestHandleRequestStatus_DuplicateEventIDIsNoOp(t *testing.T) {
	h := newRequestHarness()
	h.processed.seen["ev-1"] = true
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.calls != 0 || h.state.calls != 0 || len(h.obs.results) != 0 {
		t.Errorf("duplicate touched things: tracker=%d state=%d results=%v", h.tracker.calls, h.state.calls, h.obs.results)
	}
}

func TestHandleRequestStatus_SecondDeliveryOfSameEventDoesNotRepeatJira(t *testing.T) {
	h := newRequestHarness()
	ev := requestEvent("ev-1", "bug", "executing")
	for i := 0; i < 2; i++ {
		if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if h.tracker.calls != 1 {
		t.Errorf("transitions = %d, want 1", h.tracker.calls)
	}
}

func TestHandleRequestStatus_StaleVersionIsSkipped(t *testing.T) {
	h := newRequestHarness()
	newer := requestEvent("ev-new", "bug", "executing")
	newer.Version = 5
	older := requestEvent("ev-old", "bug", "executing")
	older.Version = 3
	for _, ev := range []domain.RequestStatusEvent{newer, older} {
		if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if h.tracker.calls != 1 {
		t.Errorf("transitions = %d, want 1 (older version must be dropped)", h.tracker.calls)
	}
	if want := []string{"status_changed:applied", "status_changed:skipped_stale"}; !reflect.DeepEqual(h.obs.results, want) {
		t.Errorf("results = %v, want %v", h.obs.results, want)
	}
	if !h.processed.seen["ev-old"] {
		t.Error("a stale event must be acknowledged")
	}
}

func TestHandleRequestStatus_NonJiraSourcesAreSkipped(t *testing.T) {
	for _, provider := range []string{"github", "gitlab", "linear", "mcp", "manual", ""} {
		h := newRequestHarness()
		ev := requestEvent("ev-1", "bug", "executing")
		ev.SourceProvider = provider
		if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
		if h.tracker.calls != 0 || h.scm.calls != 0 || h.state.calls != 0 {
			t.Errorf("provider %q must touch nothing", provider)
		}
		if !reflect.DeepEqual(h.obs.results, []string{"status_changed:skipped_not_jira"}) {
			t.Errorf("provider %q results = %v", provider, h.obs.results)
		}
	}
}

func TestHandleRequestStatus_ActorSelection(t *testing.T) {
	cases := []struct {
		name, actorID, kind, reporter, wantUser string
	}{
		{"user actor", "user-7", "user", "user-9", "user-7"},
		{"ai actor falls back to reporter", "agent-1", "ai", "user-9", "user-9"},
		{"system actor falls back to reporter", "", "system", "user-9", "user-9"},
		{"user kind without id falls back", "", "user", "user-9", "user-9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRequestHarness()
			ev := requestEvent("ev-1", "bug", "executing")
			ev.ActorID, ev.ActorKind, ev.ReporterID = tc.actorID, tc.kind, tc.reporter
			if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
				t.Fatal(err)
			}
			if h.tracker.gotUser != tc.wantUser {
				t.Errorf("user = %q, want %q", h.tracker.gotUser, tc.wantUser)
			}
		})
	}
}

func TestHandleRequestStatus_NoActorAndNoReporterIsSkippedWithoutAdvancing(t *testing.T) {
	h := newRequestHarness()
	ev := requestEvent("ev-1", "bug", "executing")
	ev.ActorID, ev.ActorKind, ev.ReporterID = "", "system", ""
	if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if h.tracker.calls != 0 || h.state.calls != 0 {
		t.Errorf("tracker=%d state=%d, want 0/0", h.tracker.calls, h.state.calls)
	}
	if !reflect.DeepEqual(h.obs.results, []string{"status_changed:skipped_no_actor"}) {
		t.Errorf("results = %v", h.obs.results)
	}
}

func TestHandleRequestStatus_AdvanceHappensBeforeJiraCall(t *testing.T) {
	h := newRequestHarness()
	h.tracker.failN = 100 // Jira keeps failing
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if h.state.last["t1/r1"] != 1 {
		t.Error("version must be recorded before the Jira call so a failure never repeats a transition")
	}
	if want := []string{"status_changed:failed"}; !reflect.DeepEqual(h.obs.results, want) {
		t.Errorf("results = %v, want %v", h.obs.results, want)
	}
	if !h.processed.seen["ev-1"] {
		t.Error("a Jira failure after retries is acknowledged")
	}
}

func TestHandleRequestStatus_TransitionUnavailableIsItsOwnResult(t *testing.T) {
	h := newRequestHarness()
	h.tracker.transitionErr = errors.New("rpc error: jira: transition unavailable: ENG-1 has no transition")
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if want := []string{"status_changed:skipped_transition_unavailable"}; !reflect.DeepEqual(h.obs.results, want) {
		t.Errorf("results = %v, want %v", h.obs.results, want)
	}
}

func TestHandleRequestStatus_SyncStateErrorNaksTwiceThenGivesUp(t *testing.T) {
	h := newRequestHarness()
	h.state.err = errors.New("db down")
	ev := requestEvent("ev-1", "bug", "executing")
	for i := 1; i <= 2; i++ {
		if err := h.uc.HandleRequestStatus(context.Background(), ev); err == nil {
			t.Fatalf("delivery %d must return an error so the event is redelivered", i)
		}
	}
	if err := h.uc.HandleRequestStatus(context.Background(), ev); err != nil {
		t.Fatalf("third delivery must give up, got %v", err)
	}
	if !h.processed.seen["ev-1"] || h.tracker.calls != 0 {
		t.Error("giving up must acknowledge without touching Jira")
	}
}

func TestHandleRequestStatus_AppliedObservesTransitionLatency(t *testing.T) {
	h := newRequestHarness()
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if h.obs.transitions != 1 || !reflect.DeepEqual(h.obs.results, []string{"status_changed:applied"}) {
		t.Errorf("transitions=%d results=%v", h.obs.transitions, h.obs.results)
	}
}

func TestHandleRequestStatus_CommentsOffByDefault(t *testing.T) {
	h := newRequestHarness() // no commenter configured
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err) // would panic on a nil commenter if it were called
	}
}

func TestHandleRequestStatus_CommentAfterTransitionAndOnBacklogCarriesNoContent(t *testing.T) {
	c := &fakeCommenter{}
	h := newRequestHarness(WithIssueComments(c, "https://orca.example/"))
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-2", "bug", "request_backlog")); err != nil {
		t.Fatal(err)
	}
	if len(c.bodies) != 2 {
		t.Fatalf("comments = %d, want 2: %v", len(c.bodies), c.bodies)
	}
	if !strings.Contains(c.bodies[0], "#42") || !strings.Contains(c.bodies[0], "https://orca.example/requests/r1") {
		t.Errorf("comment lacks number or link: %q", c.bodies[0])
	}
	if h.tracker.calls != 1 {
		t.Errorf("backlog must not transition, transitions=%d", h.tracker.calls)
	}
}

func TestHandleRequestStatus_CommentFailureOnlyCountsMetric(t *testing.T) {
	c := &fakeCommenter{err: errors.New("jira 500")}
	h := newRequestHarness(WithIssueComments(c, ""))
	if err := h.uc.HandleRequestStatus(context.Background(), requestEvent("ev-1", "bug", "executing")); err != nil {
		t.Fatal(err)
	}
	want := []string{"comment:failed", "status_changed:applied"}
	if !reflect.DeepEqual(h.obs.results, want) {
		t.Errorf("results = %v, want %v", h.obs.results, want)
	}
}

func TestDeliveryCounterIsBounded(t *testing.T) {
	c := newDeliveryCounter()
	for i := 0; i < deliveryCounterCapacity*2; i++ {
		c.next(string(rune('a'+i%26)) + time.Duration(i).String())
	}
	if len(c.counts) > deliveryCounterCapacity {
		t.Errorf("counter grew to %d", len(c.counts))
	}
}
