package usecase

import (
	"fmt"
	"sync"

	"github.com/stablyai/orca-go/common/apperrors"
)

const defaultMaxCodeIntelStreams = 16

// CodeIntelStreamLimiter bounds concurrent CodeIntel event streams per (tenant, dev_server).
// Default limit is 16 concurrent streams (PQ-18).
type CodeIntelStreamLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
}

func NewCodeIntelStreamLimiter(max int) *CodeIntelStreamLimiter {
	if max <= 0 {
		max = defaultMaxCodeIntelStreams
	}
	return &CodeIntelStreamLimiter{
		counts: make(map[string]int),
		max:    max,
	}
}

// Acquire reserves one stream slot for key (e.g. "tenantID|devServerID").
// On failure it returns an AppError with KindFailedPrecondition and code INFRA_CODEINTEL_STREAM_LIMIT.
func (l *CodeIntelStreamLimiter) Acquire(key string) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counts[key] >= l.max {
		return nil, apperrors.New(
			apperrors.KindFailedPrecondition,
			"INFRA_CODEINTEL_STREAM_LIMIT",
			fmt.Sprintf("maximum concurrent code intel streams reached (%d)", l.max),
			nil,
		)
	}
	l.counts[key]++

	var once sync.Once
	release := func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.counts[key]--
			if l.counts[key] <= 0 {
				delete(l.counts, key)
			}
		})
	}
	return release, nil
}
