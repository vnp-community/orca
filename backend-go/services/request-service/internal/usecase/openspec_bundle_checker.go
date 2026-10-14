package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// openSpecValidateTimeoutMs bounds `openspec validate`; it reads a handful of small files.
const openSpecValidateTimeoutMs = 60_000

// maxValidateFindings keeps the retry note short: the first few errors are what the agent needs.
const maxValidateFindings = 8

// OpenSpecBundleChecker is the structure gate after each authoring attempt. It reads only files of the worktree and runs
// `openspec validate`; it never asks the knowledge graph anything.
type OpenSpecBundleChecker struct {
	exec   DevServerExecutor
	conns  AnalysisConnectionResolver
	limits domain.PlanLimits
}

var _ BundleChecker = (*OpenSpecBundleChecker)(nil)

func NewOpenSpecBundleChecker(exec DevServerExecutor, conns AnalysisConnectionResolver, limits domain.PlanLimits) *OpenSpecBundleChecker {
	return &OpenSpecBundleChecker{exec: exec, conns: conns, limits: limits}
}

func (c *OpenSpecBundleChecker) Check(ctx context.Context, in BundleCheckInput) ([]BundleFinding, error) {
	if in.Workspace == nil {
		return nil, fmt.Errorf("no workspace to read the change from")
	}
	ws, changeID := in.Result.Workspace, in.Result.ChangeID
	specs, err := readSpecFiles(ctx, in.Workspace, ws, changeID)
	if err != nil {
		return nil, err
	}
	min, max := TaskRangeFor(in.Request.Size, c.limits)
	violations, err := domain.CheckBundle(domain.BundleInput{
		Proposal: in.Proposal, Specs: specs, ACIDs: activeACIDs(in.Request), MinTasks: min, MaxTasks: max,
		Probe: workspaceProbe{ctx: ctx, ws: in.Workspace, at: ws},
	})
	if err != nil {
		return nil, err
	}
	out := make([]BundleFinding, 0, len(violations))
	for _, v := range violations {
		msg := v.Message
		if v.Where != "" {
			msg = v.Where + ": " + v.Message
		}
		out = append(out, BundleFinding{Code: v.Code, Message: msg})
	}
	validation, err := c.validate(ctx, in.Request.ProjectID, ws, changeID)
	if err != nil {
		return nil, err
	}
	return append(out, validation...), nil
}

// readSpecFiles reads specs/<capability>/spec.md of the change, each capped at the proposal read limit.
func readSpecFiles(ctx context.Context, ws ProposalWorkspace, at Workspace, changeID string) (map[string]string, error) {
	root := "openspec/changes/" + changeID + "/specs"
	entries, err := ws.ListDir(ctx, at, root)
	if err != nil {
		return nil, err
	}
	specs := map[string]string{}
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		rel := root + "/" + e.Name + "/spec.md"
		info, err := ws.Stat(ctx, at, rel)
		if err != nil {
			return nil, err
		}
		if !info.Exists {
			continue
		}
		b, err := ws.ReadFile(ctx, at, rel, MaxProposalReadBytes)
		if err != nil {
			return nil, err
		}
		specs["specs/"+e.Name+"/spec.md"] = string(b)
	}
	return specs, nil
}

func activeACIDs(req domain.Request) []string {
	content, err := domain.ContentFromRequest(req)
	if err != nil {
		return nil
	}
	var ids []string
	for _, it := range content.AcceptanceCriteria.Items {
		if it.Status == domain.ACStatusActive {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// workspaceProbe answers CheckBundle's questions about the repository from the proposal worktree.
type workspaceProbe struct {
	ctx context.Context
	ws  ProposalWorkspace
	at  Workspace
}

func (p workspaceProbe) Probe(rel string) (bool, bool, error) {
	info, err := p.ws.Stat(p.ctx, p.at, rel)
	return info.Exists, info.IsDir, err
}

type validateOutput struct {
	Items []struct {
		ID     string `json:"id"`
		Valid  bool   `json:"valid"`
		Issues []struct {
			Level   string `json:"level"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"issues"`
	} `json:"items"`
}

// validate runs `openspec validate <id> --strict --json` in the worktree. A server that cannot run it is an error (the
// check did not happen), not a finding against the agent.
func (c *OpenSpecBundleChecker) validate(ctx context.Context, projectID string, ws Workspace, changeID string) ([]BundleFinding, error) {
	conn, err := c.conns.ResolveForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	res, err := c.exec.Exec(ctx, conn, ExecInput{
		Binary: "openspec", Args: []string{"validate", changeID, "--strict", "--json"}, Cwd: ws.Path, TimeoutMs: openSpecValidateTimeoutMs,
	})
	if err != nil {
		return nil, err
	}
	if res.TimedOut || res.SpawnError != "" {
		return nil, fmt.Errorf("openspec validate did not run (%s)", firstText(res.SpawnError, "timed out"))
	}
	var parsed validateOutput
	if jerr := json.Unmarshal([]byte(res.Stdout), &parsed); jerr != nil {
		if res.ExitCode == 0 {
			return nil, fmt.Errorf("openspec validate printed no readable result")
		}
		// A non-zero exit with no JSON is openspec refusing the change itself ("Unknown item ...").
		return []BundleFinding{{Code: domain.BundleValidation, Message: clip("openspec validate refused the change: "+strings.TrimSpace(firstText(res.Stdout, res.Stderr)), 300)}}, nil
	}
	var out []BundleFinding
	for _, item := range parsed.Items {
		if item.Valid {
			continue
		}
		for _, is := range item.Issues {
			if len(out) == maxValidateFindings {
				return out, nil
			}
			out = append(out, BundleFinding{Code: domain.BundleValidation, Message: clip(fmt.Sprintf("openspec validate: %s: %s", is.Path, is.Message), 300)})
		}
		if len(item.Issues) == 0 {
			out = append(out, BundleFinding{Code: domain.BundleValidation, Message: "openspec validate reports the change invalid"})
		}
	}
	return out, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func firstText(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
