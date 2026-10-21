package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Size limits of a stored review. Over any of them the review is cut, marked truncated, and says so in its warnings.
const (
	MaxReviewModelBytes = 256 * 1024
	MaxReviewItems      = 300
	MaxReviewTitleRunes = 160
	MaxReviewDetailRune = 400
	MaxReviewEvidence   = 8
)

// ReviewItemKey is stable between builds: the same section, kind and subject give the same key, so a decision survives a regeneration.
func ReviewItemKey(section ReviewSection, kind, subject string) string {
	sum := sha256.Sum256([]byte(string(section) + "|" + kind + "|" + NormalizeNFC(subject)))
	return hex.EncodeToString(sum[:])[:16]
}

// HeadKey names the code a delivered review is about: the commits of every task branch, sorted. A design review has no code.
func HeadKey(phase ReviewPhase, heads []ReviewHead) string {
	if phase == ReviewPhaseDesign {
		return ReviewDesignHeadKey
	}
	if len(heads) == 0 {
		return ReviewNoCodeHeadKey // execution is over but no task left a worktree to read
	}
	parts := make([]string, 0, len(heads))
	for _, h := range heads {
		parts = append(parts, h.Branch+"@"+h.Commit)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])[:32]
}

// Facts is everything Orca measured itself. A nil pointer or empty slice means "not read", which the build reports as a warning.
type Facts struct {
	Phase    ReviewPhase
	Subject  ReviewSubject
	Requests RequirementInputs
	// Changed is nil when the diff could not be read, and empty when it was read and nothing changed.
	Changed        []ChangedPath
	ChangedRead    bool
	Breaking       *bool
	ImpactRisk     string // low|medium|high|critical, empty when no assessment exists
	QualityGate    string // empty means absent
	Warnings       []string
	PlanTargets    []string // files the plan said it would touch
	PlanTargetsSet bool
}

// ReviewBuildInput adds what an agent reported (already validated) and the pieces computed from it.
type ReviewBuildInput struct {
	Facts      Facts
	AgentItems []ReviewItem
	// DiagramItems are rows Orca derived from the checked after-code diagrams (DiagramFindingItems); they keep their fact origin.
	DiagramItems []ReviewItem
	KG           KGInfo
	Diagrams     []DiagramRef
	Arch         DesignVerdict
	FlowVerdict  DesignVerdict
	// ExtraWarnings are warnings of the context run (a design block ignored, an edge that differed from the parser).
	ExtraWarnings []string
	// Redact removes secrets from free text; nil means no redaction, which only tests use.
	Redact func(string) string
}

// BuildOutcomeReview is a pure function of its input. The same input gives the same review byte for byte, in the same order.
func BuildOutcomeReview(requestID string, in ReviewBuildInput) (OutcomeReview, error) {
	f := in.Facts
	if !f.Phase.Valid() {
		return OutcomeReview{}, fmt.Errorf("outcome review: phase %q is not valid", f.Phase)
	}
	reqs := ClassifyRequirements(f.Requests)
	items := make([]ReviewItem, 0, len(reqs)+len(in.AgentItems)+4)
	for _, r := range reqs {
		items = append(items, r.Item)
	}
	warnings := append(append([]string(nil), f.Warnings...), in.ExtraWarnings...)

	pc := ClassifyChangedPaths(f.Changed)
	if f.Phase == ReviewPhaseDelivered {
		items = append(items, releaseItems(pc, f.Breaking)...)
		if !f.ChangedRead {
			warnings = append(warnings, "partial_facts:diff")
		}
	} else if f.PlanTargetsSet {
		items = append(items, planTargetItems(f.PlanTargets)...)
	}
	for _, it := range in.AgentItems {
		it.Origin = OriginAgent
		if !hasValidEvidence(it) {
			it.Unverified = true
		}
		items = append(items, it)
	}
	items = append(items, in.DiagramItems...)
	if !in.KG.Available {
		warnings = append(warnings, "kg_unavailable")
	}

	truncated := false
	items, truncated = finishItems(items, in.Redact)
	if truncated {
		warnings = append(warnings, "truncated:items")
	}

	flags := []string{}
	if f.Phase == ReviewPhaseDelivered && f.ChangedRead {
		flags = ReleaseFlags(pc, f.Breaking)
	}
	risk := f.ImpactRisk
	if risk == "" {
		risk = "unknown"
	}
	gate := f.QualityGate
	if gate == "" {
		gate = "absent"
	}

	out := OutcomeReview{
		SchemaVersion: OutcomeReviewSchemaVersion,
		RequestID:     requestID,
		Phase:         f.Phase,
		Subject:       f.Subject,
		State:         ReviewStateReady,
		Verdict: Verdict{
			Requirements: CountRequirements(reqs),
			Architecture: in.Arch,
			Flow:         in.FlowVerdict,
			Quality:      QualityVerdict{Gate: gate},
			Release:      ReleaseVerdict{Risk: risk, Flags: flags},
		},
		Items:     items,
		KG:        in.KG,
		Diagrams:  sortedDiagrams(in.Diagrams),
		Warnings:  dedupeSorted(warnings),
		Truncated: truncated,
	}
	digest, err := InputsDigest(out.Phase, out.Subject, f)
	if err != nil {
		return OutcomeReview{}, err
	}
	out.InputsDigest = digest
	fd, err := FactsDigest(out.Phase, f)
	if err != nil {
		return OutcomeReview{}, err
	}
	out.FactsDigest = fd
	return out, nil
}

func hasValidEvidence(it ReviewItem) bool {
	for _, e := range it.Evidence {
		if e.Kind.Valid() && strings.TrimSpace(e.Ref) != "" {
			return true
		}
	}
	return false
}

func releaseItems(pc PathClasses, breaking *bool) []ReviewItem {
	var out []ReviewItem
	if pc.Migration {
		out = append(out, ReviewItem{
			Key: ReviewItemKey(SectionData, "release", FlagMigration), Section: SectionData, Change: ChangeAdded, Origin: OriginFact,
			Title: "A database migration changed", Detail: "Deploy the migration before the service that needs it.", Severity: SeverityWarning,
		})
	}
	if pc.Config {
		out = append(out, ReviewItem{
			Key: ReviewItemKey(SectionConfig, "release", FlagConfig), Section: SectionConfig, Change: ChangeModified, Origin: OriginFact,
			Title: "A deployment file changed", Detail: "Compose, deploy or env example files differ from the base.", Severity: SeverityInfo,
		})
	}
	if breaking != nil && *breaking {
		out = append(out, ReviewItem{
			Key: ReviewItemKey(SectionContracts, "release", FlagBreaking), Section: SectionContracts, Change: ChangeModified, Origin: OriginFact,
			Title: "A contract change is breaking", Detail: "A contract check reported an incompatible change.", Severity: SeverityBreaking, NeedsDecision: true,
		})
	}
	return out
}

func planTargetItems(targets []string) []ReviewItem {
	areas := map[string]int{}
	for _, t := range targets {
		if p := normalizeRel(t); p != "" {
			areas[firstSegment(p)]++
		}
	}
	names := make([]string, 0, len(areas))
	for n := range areas {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []ReviewItem
	for _, n := range names {
		out = append(out, ReviewItem{
			Key: ReviewItemKey(SectionConfig, "plan-target", n), Section: SectionConfig, Change: ChangeUnchanged, Origin: OriginFact,
			Title: fmt.Sprintf("The plan expects to touch %d file(s) under %s", areas[n], n), Severity: SeverityInfo,
		})
	}
	return out
}

func finishItems(items []ReviewItem, redact func(string) string) ([]ReviewItem, bool) {
	seen := map[string]bool{}
	out := items[:0:0]
	for _, it := range items {
		if it.Key == "" {
			it.Key = ReviewItemKey(it.Section, "item", it.Title)
		}
		if seen[it.Key] {
			continue
		}
		seen[it.Key] = true
		it.Title = clipRunes(applyRedact(redact, it.Title), MaxReviewTitleRunes)
		it.Detail = clipRunes(applyRedact(redact, it.Detail), MaxReviewDetailRune)
		if len(it.Evidence) > MaxReviewEvidence {
			it.Evidence = it.Evidence[:MaxReviewEvidence]
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Section != b.Section {
			return sectionOrder(a.Section) < sectionOrder(b.Section)
		}
		if a.Severity.rank() != b.Severity.rank() {
			return a.Severity.rank() > b.Severity.rank()
		}
		return a.Key < b.Key
	})
	if len(out) > MaxReviewItems {
		return out[:MaxReviewItems], true
	}
	return out, false
}

func sectionOrder(s ReviewSection) int {
	for i, v := range AllReviewSections() {
		if v == s {
			return i
		}
	}
	return len(AllReviewSections())
}

func applyRedact(redact func(string) string, s string) string {
	if redact == nil {
		return s
	}
	return redact(s)
}

func clipRunes(s string, max int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\x00", "")), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

func sortedDiagrams(in []DiagramRef) []DiagramRef {
	out := append([]DiagramRef{}, in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Section != out[j].Section {
			return sectionOrder(out[i].Section) < sectionOrder(out[j].Section)
		}
		return out[i].Variant < out[j].Variant
	})
	return out
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// InputsDigest hashes what the facts were built from. It holds no time, no run id and nothing an agent said, so a design
// review keeps its digest until the criteria or the plan change.
func InputsDigest(phase ReviewPhase, subject ReviewSubject, f Facts) (string, error) {
	type ac struct{ ID, Text, Status string }
	type digestInput struct {
		Schema   int
		Phase    ReviewPhase
		BaseRef  string
		Heads    []ReviewHead
		Criteria []ac
		Coverage []CoverageRow
		Tasks    map[string]CoveringTask
		DataOK   bool
		Changed  []ChangedPath
		Read     bool
		Breaking *bool
		Impact   string
		Gate     string
		Targets  []string
	}
	d := digestInput{
		Schema: OutcomeReviewSchemaVersion, Phase: phase, BaseRef: subject.BaseRef, Heads: sortedHeads(subject.Heads),
		Coverage: sortedCoverage(f.Requests.Coverage), Tasks: f.Requests.Tasks, DataOK: f.Requests.DataOK,
		Changed: sortedChanged(f.Changed), Read: f.ChangedRead, Breaking: f.Breaking, Impact: f.ImpactRisk, Gate: f.QualityGate,
		Targets: sortedStrings(f.PlanTargets),
	}
	for _, c := range f.Requests.Criteria {
		d.Criteria = append(d.Criteria, ac{c.ID, NormalizeNFC(c.Text), c.Status})
	}
	sort.Slice(d.Criteria, func(i, j int) bool { return d.Criteria[i].ID < d.Criteria[j].ID })
	return DigestValue(d)
}

func sortedHeads(in []ReviewHead) []ReviewHead {
	out := append([]ReviewHead(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Branch+"@"+out[i].Commit < out[j].Branch+"@"+out[j].Commit })
	return out
}

func sortedCoverage(in []CoverageRow) []CoverageRow {
	out := append([]CoverageRow(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ACID != b.ACID {
			return a.ACID < b.ACID
		}
		if a.TaskID != b.TaskID {
			return a.TaskID < b.TaskID
		}
		return a.CheckID < b.CheckID
	})
	return out
}

func sortedChanged(in []ChangedPath) []ChangedPath {
	out := append([]ChangedPath(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
