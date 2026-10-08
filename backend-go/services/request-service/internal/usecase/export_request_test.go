package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type exportRepo struct {
	RequestRepository
	reqs []domain.Request
}

func (r *exportRepo) Get(_ context.Context, id string) (domain.Request, error) {
	for _, q := range r.reqs {
		if q.ID == id {
			return q, nil
		}
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

// List pages by index in the token, enough to prove the use case follows NextPageToken.
func (r *exportRepo) List(_ context.Context, f ListFilter) (ListResult, error) {
	start := 0
	if f.PageToken != "" {
		start, _ = strconv.Atoi(f.PageToken)
	}
	end := start + f.PageSize
	if end > len(r.reqs) {
		end = len(r.reqs)
	}
	res := ListResult{Requests: r.reqs[start:end]}
	if end < len(r.reqs) {
		res.NextPageToken = strconv.Itoa(end)
	}
	return res, nil
}

type emptyHistory struct{}

func (emptyHistory) Append(context.Context, domain.RequestTypeChange) error { return nil }
func (emptyHistory) List(context.Context, string) ([]domain.RequestTypeChange, error) {
	return nil, nil
}

type exportSolutionsFake struct {
	SolutionCoreRepository
	list []domain.Solution
}

func (f exportSolutionsFake) ListByRequestID(context.Context, string) ([]domain.Solution, error) {
	return f.list, nil
}

type fakeApprovals struct {
	ApprovalRepository
	list []domain.Approval
}

func (f fakeApprovals) List(context.Context, string, ApprovalListFilter) ([]domain.Approval, string, error) {
	return f.list, "", nil
}

type noLinks struct{ RequestLinkRepository }

func (noLinks) ListParents(context.Context, string) ([]domain.RequestLink, error)  { return nil, nil }
func (noLinks) ListChildren(context.Context, string) ([]domain.RequestLink, error) { return nil, nil }

type fakeFlags struct{ erased map[string]bool }

func (f fakeFlags) MarkSecretSuspected(context.Context, string) error { return nil }
func (f fakeFlags) Get(_ context.Context, id string) (SecurityFlags, error) {
	if f.erased[id] {
		at := time.Unix(1, 0)
		return SecurityFlags{ErasedAt: &at}, nil
	}
	return SecurityFlags{}, nil
}

func newExport(repo *exportRepo, sols []domain.Solution, flags fakeFlags) (*ExportRequest, *ExportTenantRequests, *memAuditOutbox) {
	out := &memAuditOutbox{}
	rec, _ := newRecorder(out, rollbackTx{out})
	src := ExportSources{Requests: repo, History: emptyHistory{}, Solutions: exportSolutionsFake{list: sols}, Approvals: fakeApprovals{}, Links: noLinks{}, Flags: flags}
	return NewExportRequest(src, rec, &fakeClock{now: time.Unix(10, 0)}), NewExportTenantRequests(repo, flags, rec), out
}

const ghToken = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"

func TestExportRequest_RedactsSecretsAndAudits(t *testing.T) {
	repo := &exportRepo{reqs: []domain.Request{{ID: "r1", Number: 7, Title: "t " + ghToken, Body: "key " + ghToken, Status: domain.RequestStatusCompleted, CreatedAt: time.Unix(1, 0)}}}
	sols := []domain.Solution{{ID: "s1", OptionsJSON: []byte(`{"a":"` + ghToken + `"}`)}}
	ex, _, out := newExport(repo, sols, fakeFlags{})
	b, err := ex.Execute(adminCtx(tA), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.JSON, "ghp_abcdef") || !json.Valid([]byte(b.JSON)) || b.Bytes != int64(len(b.JSON)) {
		t.Fatalf("bundle must be valid JSON without the token: %s", b.JSON)
	}
	if !strings.Contains(b.JSON, `"patterns_version":"ss/1"`) {
		t.Error("patterns version missing")
	}
	if len(out.rows) != 1 || out.rows[0].Action != domain.AuditRequestExport {
		t.Fatalf("export audit: %+v", out.rows)
	}
}

func TestExportRequest_ErasedRequestHasOnlyTheSkeleton(t *testing.T) {
	repo := &exportRepo{reqs: []domain.Request{{ID: "r1", Number: 7, Title: "should not appear", Body: "nor this", Status: domain.RequestStatusCompleted}}}
	ex, _, _ := newExport(repo, nil, fakeFlags{erased: map[string]bool{"r1": true}})
	b, err := ex.Execute(adminCtx(tA), "r1")
	if err != nil || strings.Contains(b.JSON, "should not appear") || strings.Contains(b.JSON, "nor this") || !strings.Contains(b.JSON, `"erased":true`) {
		t.Fatalf("%v %s", err, b.JSON)
	}
}

func TestExportRequest_TooLargeFailsInsteadOfCutting(t *testing.T) {
	repo := &exportRepo{reqs: []domain.Request{{ID: "r1", Status: domain.RequestStatusCompleted}}}
	huge := []domain.Solution{{ID: "s1", OptionsJSON: []byte(`{"x":"` + strings.Repeat("ab ", 1<<20) + `"}`)}, {ID: "s2", OptionsJSON: []byte(`{"x":"` + strings.Repeat("cd ", 1<<20) + `"}`)}}
	ex, _, out := newExport(repo, huge, fakeFlags{})
	_, err := ex.Execute(adminCtx(tA), "r1")
	if err == nil || !strings.Contains(err.Error(), "REQUEST_EXPORT_TOO_LARGE") {
		t.Fatalf("got %v", err)
	}
	if len(out.rows) != 0 {
		t.Error("nothing was exported, so nothing is audited as exported")
	}
}

func TestExportRequest_AdminOnlyAndAuditFailureBlocksExport(t *testing.T) {
	repo := &exportRepo{reqs: []domain.Request{{ID: "r1", Status: domain.RequestStatusCompleted}}}
	ex, _, out := newExport(repo, nil, fakeFlags{})
	member := tenant.WithUserID(tenant.WithTenantID(context.Background(), tA), "u")
	if _, err := ex.Execute(member, "r1"); err == nil || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") {
		t.Errorf("non admin: %v", err)
	}
	if _, err := ex.Execute(tenant.WithActorType(adminCtx(tA), "agent"), "r1"); err == nil {
		t.Error("agent behind admin must be refused")
	}
	out.failEnq = errors.New("down")
	if _, err := ex.Execute(adminCtx(tA), "r1"); err == nil {
		t.Error("an export that cannot be audited must not be handed out")
	}
}

func makeRequests(n int, bodyFn func(i int) string) []domain.Request {
	out := make([]domain.Request, n)
	for i := range out {
		out[i] = domain.Request{ID: "r" + strconv.Itoa(i), Number: int64(i), Title: "t" + strconv.Itoa(i), Body: bodyFn(i), Status: domain.RequestStatusCompleted, CreatedAt: time.Unix(int64(i), 0)}
	}
	return out
}

func TestExportTenantRequests_StreamsAllPagesInChunksUnder64KB(t *testing.T) {
	bigBody := strings.Repeat("<é> ", 20000) // escaping-prone, ~100k characters
	repo := &exportRepo{reqs: makeRequests(250, func(i int) string {
		if i == 3 {
			return bigBody + ghToken
		}
		return "body"
	})}
	_, all, out := newExport(repo, nil, fakeFlags{})
	var lines []string
	n, err := all.Run(adminCtx(tA), "", func(line string, idx int64) error {
		if idx != int64(len(lines)) {
			t.Fatalf("index %d for line %d", idx, len(lines))
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil || n != 250 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	seen := map[string]bool{}
	var joined strings.Builder
	for _, l := range lines {
		if len(l) > ExportChunkBytes {
			t.Fatalf("chunk of %d bytes exceeds 64 KB", len(l))
		}
		if !strings.HasSuffix(l, "\n") || !json.Valid([]byte(l)) {
			t.Fatalf("each chunk is one JSON line: %q", l[:40])
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(l), &m)
		if id, _ := m["id"].(string); id != "" {
			seen[id] = true
		}
		joined.WriteString(l)
	}
	if len(seen) != 250 || len(lines) <= 250 {
		t.Errorf("all Requests present and the big body split into extra lines: seen=%d lines=%d", len(seen), len(lines))
	}
	if strings.Contains(joined.String(), "ghp_abcdef") {
		t.Error("secrets must be masked in the stream")
	}
	if len(out.rows) != 1 || !strings.Contains(out.rows[0].MetadataJSON, `"complete":true`) || !strings.Contains(out.rows[0].MetadataJSON, `"requests":250`) {
		t.Errorf("audit: %+v", out.rows)
	}
}

func TestExportTenantRequests_CancelStopsEarlyAndStillAudits(t *testing.T) {
	repo := &exportRepo{reqs: makeRequests(500, func(int) string { return "b" })}
	_, all, out := newExport(repo, nil, fakeFlags{})
	ctx, cancel := context.WithCancel(adminCtx(tA))
	sent := 0
	n, err := all.Run(ctx, "", func(string, int64) error {
		sent++
		if sent == 120 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || n >= 500 || sent > 130 {
		t.Fatalf("n=%d sent=%d err=%v", n, sent, err)
	}
	if len(out.rows) != 1 || !strings.Contains(out.rows[0].MetadataJSON, `"complete":false`) {
		t.Errorf("a cancelled export is still audited as incomplete: %+v", out.rows)
	}
}

func TestExportTenantRequests_OneAtATimePerTenantAndAdminOnly(t *testing.T) {
	repo := &exportRepo{reqs: makeRequests(1, func(int) string { return "b" })}
	_, all, _ := newExport(repo, nil, fakeFlags{})
	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := all.Run(adminCtx(tA), "", func(string, int64) error {
			close(started)
			<-release
			return nil
		})
		done <- err
	}()
	<-started
	if _, err := all.Run(adminCtx(tA), "", func(string, int64) error { return nil }); err == nil || !strings.Contains(err.Error(), "REQUEST_EXPORT_IN_PROGRESS") {
		t.Errorf("second export of the same tenant: %v", err)
	}
	if _, err := all.Run(adminCtx(tB), "", func(string, int64) error { return nil }); err != nil {
		t.Errorf("another tenant is independent: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := all.Run(tenant.WithTenantID(context.Background(), tA), "", func(string, int64) error { return nil }); err == nil {
		t.Error("non admin must be refused")
	}
}

func TestExportTenantRequests_ErasedRequestsAreSkeletons(t *testing.T) {
	repo := &exportRepo{reqs: makeRequests(2, func(int) string { return "SECRET-BODY" })}
	_, all, _ := newExport(repo, nil, fakeFlags{erased: map[string]bool{"r0": true}})
	var sb strings.Builder
	if _, err := all.Run(adminCtx(tA), "", func(l string, _ int64) error { sb.WriteString(l); return nil }); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(sb.String()), "\n")
	if strings.Contains(lines[0], "SECRET-BODY") || !strings.Contains(lines[1], "SECRET-BODY") {
		t.Errorf("erased first, live second: %v", lines)
	}
}
