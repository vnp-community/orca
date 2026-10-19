package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	beginRegex = regexp.MustCompile(`^<!--\s*orca:begin\s+plan\s+request=([a-zA-Z0-9-]+)\s+schema=1\s*-->$`)
	endRegex   = regexp.MustCompile(`^<!--\s*orca:end\s+plan\s*-->$`)
	phaseRegex = regexp.MustCompile(`^##\s+PH-(\d+)\s+(.+)$`)
	tasksRegex = regexp.MustCompile(`^##\s+TASKS\s*$`)
	taskRegex  = regexp.MustCompile(`^-\s+\[(.)\]\s+T(\d+)\.(\d+)\s+(.+?)(?:\s+((?:\[[a-zA-Z0-9_]+(?:=[^\]]*)?\]\s*)*))?$`)
	attrRegex  = regexp.MustCompile(`\[([a-zA-Z0-9_]+)(?:=([^\]]*))?\]`)
)

func ParsePlanRegion(md string, wantRequest string) (PlanProposal, []Violation) {
	var p PlanProposal
	var violations []Violation

	t := transform.Chain(norm.NFC)
	md, _, _ = transform.String(t, md)
	md = strings.ReplaceAll(md, "\r\n", "\n")

	if len(md) > 1024*1024 {
		return p, []Violation{{Line: 1, Code: "TASKSMD_TOO_LARGE", Message: "file too large"}}
	}

	lines := strings.Split(md, "\n")

	startIdx := -1
	endIdx := -1
	requestID := ""

	for i, line := range lines {
		if beginRegex.MatchString(line) {
			if startIdx != -1 {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_REGION_DUPLICATE", Message: "duplicate begin"})
				return p, violations
			}
			startIdx = i
			matches := beginRegex.FindStringSubmatch(line)
			requestID = matches[1]
		} else if endRegex.MatchString(line) {
			if startIdx == -1 {
				continue
			}
			endIdx = i
			break
		}
	}

	if startIdx == -1 || endIdx == -1 {
		violations = append(violations, Violation{Line: 1, Code: "TASKSMD_REGION_MISSING", Message: "missing region"})
		return p, violations
	}
	// A second region after the first would be silently ignored, so it is refused: one document, one plan.
	for i := endIdx + 1; i < len(lines); i++ {
		if beginRegex.MatchString(lines[i]) {
			violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_REGION_DUPLICATE", Message: "duplicate begin"})
			return p, violations
		}
	}

	if requestID != wantRequest {
		violations = append(violations, Violation{Line: startIdx + 1, Code: "TASKSMD_REQUEST_MISMATCH", Message: "request mismatch"})
	}

	var currentPhase *PhaseProposal
	expectedPhase := 1
	hasTasksRegion := false

	// A task is addressed by (phase, pos), never by pointer: appends reallocate the slices, and a pointer taken earlier
	// would write into a copy that is no longer part of the proposal. phase is -1 for the flat ## TASKS container.
	type taskRef struct {
		phase, pos int
		deps       []string
		line       int
	}
	var allTasks []*taskRef
	idMap := make(map[string]*taskRef)
	taskAt := func(r *taskRef) *TaskProposal {
		if r.phase < 0 {
			return &p.Tasks[r.pos]
		}
		return &p.Phases[r.phase].Tasks[r.pos]
	}

	for i := startIdx + 1; i < endIdx; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, "<!-- orca:task") {
			// ignore task annotations
			continue
		}

		if phaseRegex.MatchString(line) {
			if hasTasksRegion {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_MIXED_CHILDREN", Message: "mixed tasks and phases"})
				continue
			}
			matches := phaseRegex.FindStringSubmatch(line)
			num, _ := strconv.Atoi(matches[1])
			if num != expectedPhase {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_PHASE_ORDER", Message: "phase out of order"})
			}
			expectedPhase++
			p.Phases = append(p.Phases, PhaseProposal{Title: strings.TrimSpace(matches[2])})
			currentPhase = &p.Phases[len(p.Phases)-1]
		} else if tasksRegex.MatchString(line) {
			if len(p.Phases) > 0 {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_MIXED_CHILDREN", Message: "mixed tasks and phases"})
				continue
			}
			hasTasksRegion = true
		} else if strings.HasPrefix(line, "> ") {
			if currentPhase != nil {
				currentPhase.Description += strings.TrimPrefix(line, "> ") + "\n"
			}
		} else if taskRegex.MatchString(line) {
			matches := taskRegex.FindStringSubmatch(line)
			done := matches[1] == "x" || matches[1] == "X"
			if done {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_PREDONE_TASK", Message: "task is already done"})
			} else if matches[1] != " " {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_BAD_CHECKBOX", Message: "a checkbox is [ ] or [x]"})
			}

			pId := matches[2]
			qId := matches[3]
			// Ids are positional (T<phase>.<n>, both counted from 1) because the renderer regenerates them and
			// depends= points at them: a wrong or repeated id would silently re-point a dependency.
			wantP, wantQ := 1, len(p.Tasks)+1
			if currentPhase != nil {
				wantP, wantQ = len(p.Phases), len(currentPhase.Tasks)+1
			}
			if pId != strconv.Itoa(wantP) || qId != strconv.Itoa(wantQ) {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_ID_ORDER", Message: fmt.Sprintf("expected id T%d.%d, got T%s.%s", wantP, wantQ, pId, qId)})
			}
			if currentPhase == nil && !hasTasksRegion {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_TASK_OUTSIDE_CONTAINER", Message: "a task needs a ## PH-<n> or ## TASKS heading above it"})
			}
			title := strings.TrimSpace(matches[4])
			if utf8.RuneCountInString(title) > 200 {
				violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_TITLE_TOO_LONG", Message: "title too long"})
				title = string([]rune(title)[:200])
			}

			tp := TaskProposal{
				Title: title,
				Done:  done,
			}

			deps := []string{}

			attrs := matches[5]
			if attrs != "" {
				attrMatches := attrRegex.FindAllStringSubmatch(attrs, -1)
				for _, am := range attrMatches {
					k := am[1]
					v := am[2]
					switch k {
					case "type":
						tp.TaskType = v
					case "h":
						h, herr := strconv.ParseFloat(v, 64)
						if herr != nil || h < 0 {
							violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_BAD_ATTR", Message: "h must be a non-negative number, got " + v})
						}
						tp.EstimatedHours = h
					case "ac":
						tp.Satisfies = strings.Split(v, ",")
					case "labels":
						tp.Labels = strings.Split(v, ",")
					case "depends":
						deps = strings.Split(v, ",")
					case "irreversible":
						tp.Irreversible = true
					default:
						violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_UNKNOWN_ATTR", Message: "unknown attr " + k})
					}
				}
			}

			var tr *taskRef
			if currentPhase != nil {
				currentPhase.Tasks = append(currentPhase.Tasks, tp)
				tr = &taskRef{phase: len(p.Phases) - 1, pos: len(currentPhase.Tasks) - 1, deps: deps, line: i + 1}
			} else {
				p.Tasks = append(p.Tasks, tp)
				tr = &taskRef{phase: -1, pos: len(p.Tasks) - 1, deps: deps, line: i + 1}
			}
			allTasks = append(allTasks, tr)
			idMap[fmt.Sprintf("T%s.%s", pId, qId)] = tr
		} else if strings.HasPrefix(line, "  ") {
			if len(allTasks) > 0 {
				lastTask := taskAt(allTasks[len(allTasks)-1])
				lastTask.Description += strings.TrimPrefix(line, "  ") + "\n"
			}
		} else {
			violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_UNKNOWN_LINE", Message: "unknown line"})
		}
	}

	// Plan semantics: depends_on_indices point at siblings of the same container; a dependency on an earlier phase is
	// the phase's own dependency (Phase.DependsOnPhaseIndices), and phases are linearly ordered ("a phase may only rely on
	// earlier ones"), so a phase also starts after the one above it.
	phaseDeps := make([]map[int]bool, len(p.Phases))
	for i := range phaseDeps {
		phaseDeps[i] = map[int]bool{}
		if i > 0 {
			phaseDeps[i][i-1] = true
		}
	}
	for _, tr := range allTasks {
		for _, dep := range tr.deps {
			ref, ok := idMap[dep]
			switch {
			case !ok:
				violations = append(violations, Violation{Line: tr.line, Code: "TASKSMD_DEPENDENCY_UNKNOWN", Message: "unknown dependency " + dep})
			case ref.phase == tr.phase:
				t := taskAt(tr)
				t.DependsOnIndices = append(t.DependsOnIndices, ref.pos)
			case ref.phase < tr.phase:
				phaseDeps[tr.phase][ref.phase] = true
			default:
				violations = append(violations, Violation{Line: tr.line, Code: "TASKSMD_DEPENDENCY_FORWARD", Message: "a task may depend only on tasks of its own phase or of earlier phases: " + dep})
			}
		}
	}
	for i := range p.Phases {
		for j := range phaseDeps[i] {
			p.Phases[i].DependsOnPhaseIndices = append(p.Phases[i].DependsOnPhaseIndices, j)
		}
		sort.Ints(p.Phases[i].DependsOnPhaseIndices)
	}

	// A cycle can only be inside one container (the rest point backwards), so each container is checked on its own.
	cycleLines := map[[2]int]int{}
	for _, tr := range allTasks {
		cycleLines[[2]int{tr.phase, tr.pos}] = tr.line
	}
	check := func(phase int, tasks []TaskProposal) {
		state := make([]int, len(tasks)) // 0 new, 1 on the stack, 2 done
		var visit func(n int) bool
		visit = func(n int) bool {
			if state[n] == 1 {
				return true
			}
			if state[n] == 2 {
				return false
			}
			state[n] = 1
			for _, d := range tasks[n].DependsOnIndices {
				if visit(d) {
					return true
				}
			}
			state[n] = 2
			return false
		}
		for n := range tasks {
			if visit(n) {
				violations = append(violations, Violation{Line: cycleLines[[2]int{phase, n}], Code: "TASKSMD_DEPENDENCY_CYCLE", Message: "dependency cycle detected"})
				return
			}
		}
	}
	check(-1, p.Tasks)
	for i := range p.Phases {
		check(i, p.Phases[i].Tasks)
	}

	return p, violations
}
