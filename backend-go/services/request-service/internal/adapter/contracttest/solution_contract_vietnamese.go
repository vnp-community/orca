package contracttest

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// vietnameseText returns exactly n runes of precomposed Vietnamese letters; most are 3 bytes in UTF-8.
func vietnameseText(n int) string {
	cycle := []rune("ệếưỡạờ ")
	out := make([]rune, n)
	for i := range out {
		out[i] = cycle[i%len(cycle)]
	}
	return string(out)
}

// solLargeVietnameseDocument writes the biggest Solution the validator accepts, in accented text, and a run output at its
// cap, so a column that is too small or a lossy charset shows up on both dialects (CR-REQ-046).
func solLargeVietnameseDocument(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	opts, err := domain.ParseSolutionOptions([]byte(validSolutionDoc(t)))
	if err != nil {
		t.Fatal(err)
	}
	base := opts.Options[1]
	opts.Options = opts.Options[:1]
	for i := 2; i <= 4; i++ {
		o := base
		o.ID = "opt-" + string(rune('0'+i))
		o.Recommended = false
		opts.Options = append(opts.Options, o)
	}
	for i := range opts.Options {
		opts.Options[i].Title = vietnameseText(120)
		opts.Options[i].Summary = vietnameseText(600)
		opts.Options[i].Approach = vietnameseText(4000)
	}
	if err := opts.Validate(2); err != nil {
		t.Fatalf("the fixture must be a valid document: %v", err)
	}
	doc, err := opts.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc) > domain.MaxOptionsBytes {
		t.Fatalf("the fixture is %d bytes, over the %d byte document limit", len(doc), domain.MaxOptionsBytes)
	}
	before, err := domain.DigestOptions(doc, nil)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	s := domain.Solution{ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusProposed,
		OptionsJSON: doc, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := env.SolutionStore.Insert(k.ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := env.SolutionStore.Get(k.ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	// MySQL's JSON type may reorder keys and respace, so the document is compared by its canonical digest and its text.
	if after, err := domain.DigestOptions(got.OptionsJSON, nil); err != nil || after != before {
		t.Fatalf("digest changed through the database: %s -> %s (%v)", before, after, err)
	}
	back, err := domain.ParseSolutionOptions(got.OptionsJSON)
	if err != nil || back.Validate(2) != nil {
		t.Fatalf("stored document no longer valid: %v", err)
	}
	for i, o := range back.Options {
		if o.Approach != vietnameseText(4000) || o.Title != vietnameseText(120) || o.Summary != vietnameseText(600) {
			t.Fatalf("option %d text changed through the database", i)
		}
	}

	// A run output at the 256 KB cap, in 3-byte letters so the cut falls inside a character unless it is made on a boundary.
	raw := domain.TruncateRaw(strings.Repeat(vietnameseText(7), 40_000))
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	res, err := startIn(t, k, run, draft, 0)
	if err != nil {
		t.Fatal(err)
	}
	done := res.Run
	done.Succeed(time.Now())
	done.RawOutput = &raw
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, done, "w-test"); err != nil || !ok {
		t.Fatalf("finish: %v %v", ok, err)
	}
	stored, err := env.AnalysisRuns.Get(k.ctx, res.Run.ID)
	if err != nil || stored.RawOutput == nil || *stored.RawOutput != raw {
		t.Fatalf("raw output of %d bytes changed through the database: %v", len(raw), err)
	}
}
