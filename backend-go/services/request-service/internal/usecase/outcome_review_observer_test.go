package usecase

import (
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// reviewMetricsLog records what the review reports, as the metrics adapter would count it.
type reviewMetricsLog struct {
	NoopEngineObserver
	mu                       sync.Mutex
	hits, misses             int
	runs                     []string
	tokens                   int64
	costErr                  []float64
	drawn, failed            int
	regen                    []string
	firstFacts               int
	itemsTotal, itemsUnverif int
	designRatio              []float64
}

func (m *reviewMetricsLog) ReviewContextLookup(hit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hit {
		m.hits++
	} else {
		m.misses++
	}
}
func (m *reviewMetricsLog) ReviewRunEnded(kind, result string, _ time.Duration) {
	m.mu.Lock()
	m.runs = append(m.runs, kind+"/"+result)
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewTokens(_ string, in, out int64) {
	m.mu.Lock()
	m.tokens += in + out
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewCostEstimateError(_ string, r float64) {
	m.mu.Lock()
	m.costErr = append(m.costErr, r)
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewDrawn(d, f int, _ string) {
	m.mu.Lock()
	m.drawn += d
	m.failed += f
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewRegenerated(r string) {
	m.mu.Lock()
	m.regen = append(m.regen, r)
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewFirstFacts(time.Duration) {
	m.mu.Lock()
	m.firstFacts++
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewItems(t, u int) {
	m.mu.Lock()
	m.itemsTotal += t
	m.itemsUnverif += u
	m.mu.Unlock()
}
func (m *reviewMetricsLog) ReviewDesignReviewed(r float64) {
	m.mu.Lock()
	m.designRatio = append(m.designRatio, r)
	m.mu.Unlock()
}

func TestReviewObserver_ContextRunReportsCacheRunAndItems(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	m := &reviewMetricsLog{}
	e.svc.Observer, e.runner.Observer = m, m
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}

	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	if m.firstFacts != 1 || m.misses != 1 || m.hits != 0 || len(m.runs) != 1 || m.runs[0] != "context/succeeded" {
		t.Errorf("first ask: %+v", m)
	}
	if m.itemsTotal == 0 || m.itemsUnverif > m.itemsTotal {
		t.Errorf("items: total=%d unverified=%d", m.itemsTotal, m.itemsUnverif)
	}
	if len(m.designRatio) != 1 || m.designRatio[0] != 0.5 {
		t.Errorf("one of the two elements read is approved: %v", m.designRatio)
	}

	// The second ask finds the checked result: one more hit, no new run, and no second "first facts" (the review already exists).
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	if m.hits != 1 || len(m.runs) != 1 || m.firstFacts != 1 {
		t.Errorf("second ask: hits=%d runs=%v firstFacts=%d", m.hits, m.runs, m.firstFacts)
	}
}

func TestReviewObserver_CostErrorNeedsAPriorRunToPredictFrom(t *testing.T) {
	m := &reviewMetricsLog{}
	run := ReviewRun{Kind: ReviewRunContext, CostEstimate: "0.4000 USD", StartedAt: time.Unix(100, 0)}
	observeRunEnd(m, run, ReviewRunSucceeded, time.Unix(160, 0), &domain.RunUsage{InputTokens: 1000, OutputTokens: 200}, "0.5000 USD")
	if len(m.costErr) != 1 || m.costErr[0] < 0.249 || m.costErr[0] > 0.251 || m.tokens != 1200 {
		t.Errorf("0.40 predicted and 0.50 spent is a 25%% error: %v tokens=%d", m.costErr, m.tokens)
	}
	observeRunEnd(m, ReviewRun{Kind: ReviewRunContext, StartedAt: time.Unix(100, 0)}, ReviewRunSucceeded, time.Unix(160, 0), nil, "0.5000 USD")
	observeRunEnd(m, run, ReviewRunFailed, time.Unix(160, 0), nil, "0.5000 USD")
	if len(m.costErr) != 1 {
		t.Errorf("no prediction, or a failed run, measures nothing: %v", m.costErr)
	}
	if len(m.runs) != 3 {
		t.Errorf("every run that ends is counted: %v", m.runs)
	}
}

func TestReviewObserver_NoObserverIsFine(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	reviewObserverOf(nil).ReviewItems(1, 1)
	reviewObserverOf(NoopEngineObserver{}).ReviewDesignReviewed(1)
}
