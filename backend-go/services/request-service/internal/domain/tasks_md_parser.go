package domain

import (
	"fmt"
	"regexp"
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

	if requestID != wantRequest {
		violations = append(violations, Violation{Line: startIdx + 1, Code: "TASKSMD_REQUEST_MISMATCH", Message: "request mismatch"})
	}

	var currentPhase *PhaseProposal
	expectedPhase := 1
	hasTasksRegion := false

	type taskRef struct {
		prop   *TaskProposal
		deps   []string
		line   int
	}
	var allTasks []*taskRef
	idMap := make(map[string]int)

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
			}
			
			pId := matches[2]
			qId := matches[3]
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
						h, _ := strconv.ParseFloat(v, 64)
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

			if currentPhase != nil {
				currentPhase.Tasks = append(currentPhase.Tasks, tp)
				tr := &taskRef{prop: &currentPhase.Tasks[len(currentPhase.Tasks)-1], deps: deps, line: i + 1}
				allTasks = append(allTasks, tr)
				idMap[fmt.Sprintf("T%s.%s", pId, qId)] = len(allTasks) - 1
			} else {
				p.Tasks = append(p.Tasks, tp)
				tr := &taskRef{prop: &p.Tasks[len(p.Tasks)-1], deps: deps, line: i + 1}
				allTasks = append(allTasks, tr)
				idMap[fmt.Sprintf("T%s.%s", pId, qId)] = len(allTasks) - 1
			}
		} else if strings.HasPrefix(line, "  ") {
			if len(allTasks) > 0 {
				lastTask := allTasks[len(allTasks)-1].prop
				lastTask.Description += strings.TrimPrefix(line, "  ") + "\n"
			}
		} else {
			violations = append(violations, Violation{Line: i + 1, Code: "TASKSMD_UNKNOWN_LINE", Message: "unknown line"})
		}
	}

	for _, tr := range allTasks {
		for _, dep := range tr.deps {
			idx, ok := idMap[dep]
			if !ok {
				violations = append(violations, Violation{Line: tr.line, Code: "TASKSMD_DEPENDENCY_UNKNOWN", Message: "unknown dependency " + dep})
			} else {
				tr.prop.DependsOnIndices = append(tr.prop.DependsOnIndices, idx)
			}
		}
	}

	// cycle detection simplified
	for i, tr := range allTasks {
		visited := make(map[int]bool)
		var checkCycle func(int) bool
		checkCycle = func(n int) bool {
			if visited[n] {
				return true
			}
			visited[n] = true
			for _, dep := range allTasks[n].prop.DependsOnIndices {
				if checkCycle(dep) {
					return true
				}
			}
			visited[n] = false
			return false
		}
		if checkCycle(i) {
			violations = append(violations, Violation{Line: tr.line, Code: "TASKSMD_DEPENDENCY_CYCLE", Message: "dependency cycle detected"})
		}
	}

	return p, violations
}
