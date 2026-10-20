package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// fakeRevisions keeps the versions of a change in memory with the same rules as the real repositories: a number or a run
// is stored once, and Append drops the text of the versions that fall out of the window.
type fakeRevisions struct {
	mu   sync.Mutex
	rows []domain.OpenSpecChangeRevision
	err  error
}

var _ OpenSpecChangeRevisionRepository = (*fakeRevisions)(nil)

func (f *fakeRevisions) Append(_ context.Context, rev domain.OpenSpecChangeRevision, keepText int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	for _, r := range f.rows {
		if r.RequestID == rev.RequestID && r.Revision == rev.Revision || r.RunID == rev.RunID {
			return fmt.Errorf("duplicate version %d of %s", rev.Revision, rev.RequestID)
		}
	}
	rev.TextKept = rev.Bundle != nil
	f.rows = append(f.rows, rev)
	if keepText > 0 {
		for i := range f.rows {
			if f.rows[i].RequestID == rev.RequestID && f.rows[i].Revision <= rev.Revision-keepText {
				f.rows[i].Bundle, f.rows[i].TextKept = nil, false
			}
		}
	}
	return nil
}

func (f *fakeRevisions) List(_ context.Context, requestID string, before, limit int) ([]domain.OpenSpecChangeRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.OpenSpecChangeRevision
	for _, r := range f.rows {
		if r.RequestID == requestID && (before <= 0 || r.Revision < before) {
			r.Bundle = nil
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision > out[j].Revision })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeRevisions) Get(_ context.Context, requestID string, revision int) (domain.OpenSpecChangeRevision, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.RequestID == requestID && r.Revision == revision {
			return r, true, nil
		}
	}
	return domain.OpenSpecChangeRevision{}, false, nil
}

func (f *fakeRevisions) GetByRun(_ context.Context, runID string) (domain.OpenSpecChangeRevision, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.RunID == runID {
			return r, true, nil
		}
	}
	return domain.OpenSpecChangeRevision{}, false, nil
}

var errRevisionStore = errors.New("revision store unavailable")
