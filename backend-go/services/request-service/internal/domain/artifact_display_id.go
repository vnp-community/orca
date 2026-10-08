package domain

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/stablyai/orca-go/common/apperrors"
)

// DisplayKind is the entity a display id names. Display ids are stable aliases; the real key stays a UUID.
type DisplayKind string

const (
	DisplayKindRequest       DisplayKind = "request"
	DisplayKindAC            DisplayKind = "ac"
	DisplayKindSolution      DisplayKind = "solution"
	DisplayKindOption        DisplayKind = "option"
	DisplayKindPlan          DisplayKind = "plan"
	DisplayKindPhase         DisplayKind = "phase"
	DisplayKindTask          DisplayKind = "task"
	DisplayKindClarification DisplayKind = "clarification"
	DisplayKindDecision      DisplayKind = "decision"
)

// DisplayID is a parsed display id. Seq is the solution/plan/clarification/decision sequence,
// Index the phase/task/option/AC ordinal within it.
type DisplayID struct {
	Kind   DisplayKind
	ReqNum int64
	Seq    int
	Index  int
}

func FormatRequestID(n int64) string { return fmt.Sprintf("REQ-%d", n) }

// FormatAC accepts the stored form "AC-3" and returns "REQ-142#AC-3".
func FormatAC(reqNum int64, ac string) string { return fmt.Sprintf("REQ-%d#%s", reqNum, ac) }

func FormatSolutionID(reqNum int64, seq int) string { return fmt.Sprintf("SOL-%d.%d", reqNum, seq) }

func FormatOptionID(solutionID string, k int) string { return fmt.Sprintf("%s/opt-%d", solutionID, k) }

func FormatPlanID(reqNum int64, seq int) string { return fmt.Sprintf("PLN-%d.%d", reqNum, seq) }

func FormatPhaseID(reqNum int64, planSeq, k int) string {
	return fmt.Sprintf("PH-%d.%d.%d", reqNum, planSeq, k)
}

func FormatTaskID(reqNum int64, planSeq, k int) string {
	return fmt.Sprintf("TSK-%d.%d.%d", reqNum, planSeq, k)
}

func FormatClarificationID(reqNum int64, seq int) string {
	return fmt.Sprintf("CLR-%d.%d", reqNum, seq)
}

func FormatDecisionID(reqNum int64, seq int) string { return fmt.Sprintf("DEC-%d.%d", reqNum, seq) }

var displayIDPatterns = []struct {
	kind DisplayKind
	re   *regexp.Regexp
}{
	{DisplayKindRequest, regexp.MustCompile(`^REQ-([1-9][0-9]*)$`)},
	{DisplayKindAC, regexp.MustCompile(`^REQ-([1-9][0-9]*)#AC-([1-9][0-9]*)$`)},
	{DisplayKindSolution, regexp.MustCompile(`^SOL-([1-9][0-9]*)\.([1-9][0-9]*)$`)},
	{DisplayKindOption, regexp.MustCompile(`^SOL-([1-9][0-9]*)\.([1-9][0-9]*)/opt-([1-9][0-9]*)$`)},
	{DisplayKindPlan, regexp.MustCompile(`^PLN-([1-9][0-9]*)\.([1-9][0-9]*)$`)},
	{DisplayKindPhase, regexp.MustCompile(`^PH-([1-9][0-9]*)\.([1-9][0-9]*)\.([1-9][0-9]*)$`)},
	{DisplayKindTask, regexp.MustCompile(`^TSK-([1-9][0-9]*)\.([1-9][0-9]*)\.([1-9][0-9]*)$`)},
	{DisplayKindClarification, regexp.MustCompile(`^CLR-([1-9][0-9]*)\.([1-9][0-9]*)$`)},
	{DisplayKindDecision, regexp.MustCompile(`^DEC-([1-9][0-9]*)\.([1-9][0-9]*)$`)},
}

func ErrArtifactIDInvalid(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ARTIFACT_ID_INVALID", fmt.Sprintf("invalid artifact id: %q", s), nil)
}

// ParseDisplayID is anchored and case-sensitive on purpose: a lookalike id must not resolve.
func ParseDisplayID(s string) (DisplayID, error) {
	for _, p := range displayIDPatterns {
		m := p.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		nums := make([]int64, 0, len(m)-1)
		for _, g := range m[1:] {
			n, err := strconv.ParseInt(g, 10, 64)
			if err != nil {
				return DisplayID{}, ErrArtifactIDInvalid(s)
			}
			nums = append(nums, n)
		}
		id := DisplayID{Kind: p.kind, ReqNum: nums[0]}
		switch p.kind {
		case DisplayKindAC:
			id.Index = int(nums[1])
		case DisplayKindSolution, DisplayKindPlan, DisplayKindClarification, DisplayKindDecision:
			id.Seq = int(nums[1])
		case DisplayKindOption:
			id.Seq, id.Index = int(nums[1]), int(nums[2])
		case DisplayKindPhase, DisplayKindTask:
			id.Seq, id.Index = int(nums[1]), int(nums[2])
		}
		return id, nil
	}
	return DisplayID{}, ErrArtifactIDInvalid(s)
}
