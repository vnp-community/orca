package resources

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

type templateDef struct {
	kind        Kind
	name, title string
	uri         string
	desc        string
	mime        string
	priority    float64
	untrusted   bool
	// sampleURI lets the policy filter build the meta without a concrete ref.
	sample string
}

const sampleUUID = "00000000-0000-4000-8000-000000000000"

var templateDefs = []templateDef{
	{kind: KindProject, name: "project", title: "Project", uri: "orca://project/{projectId}", mime: "application/json", priority: 0.6,
		desc: "A project and its worktrees.", sample: "orca://project/" + sampleUUID},
	{kind: KindTask, name: "task", title: "Task", uri: "orca://task/{taskId}", mime: "application/json", priority: 0.8, untrusted: true,
		desc: "A task with its dependencies and comments (comments are untrusted text). Subscribable.", sample: "orca://task/" + sampleUUID},
	{kind: KindWorktreeStatus, name: "worktree-status", title: "Worktree git status", uri: "orca://worktree/{worktreeId}/status", mime: "application/json", priority: 0.6,
		desc: "Git status of a worktree.", sample: "orca://worktree/" + sampleUUID + "/status"},
	{kind: KindWorktreeDiff, name: "worktree-diff", title: "Worktree file diff", uri: "orca://worktree/{worktreeId}/diff{?path,base,staged}", mime: "text/x-diff", priority: 0.7, untrusted: true,
		desc: "Unified diff of ONE file (path is required). With base, compares against that ref.", sample: "orca://worktree/" + sampleUUID + "/diff?path=a"},
	{kind: KindWorktreeFile, name: "worktree-file", title: "Worktree file", uri: "orca://worktree/{worktreeId}/file/{+path}{?offset,length}", mime: "text/plain", priority: 0.5, untrusted: true,
		desc: "Text content of a file inside a worktree. Secrets files are never served.", sample: "orca://worktree/" + sampleUUID + "/file/a"},
	{kind: KindReview, name: "review", title: "Pull / merge request", uri: "orca://review/{provider}/{repo}/{number}", mime: "application/json", priority: 0.7, untrusted: true,
		desc: "A GitHub pull request or GitLab merge request. Percent-encode repo (org%2Fsub%2Frepo).", sample: "orca://review/github/o%2Fr/1"},
}

func (r *Provider) templateVisible(ctx context.Context, p mcpserver.Principal, d templateDef) bool {
	if d.kind == KindWorktreeFile && !r.cfg.FileEnabled {
		return false
	}
	if !hasScope(p, "orca:read") {
		return false
	}
	if r.view == nil {
		return true
	}
	ref, err := Parse(d.sample)
	if err != nil {
		return false
	}
	pl, err := r.planFor(ref)
	if err != nil {
		return false
	}
	eff, err := r.view.EffectiveDecision(ctx, p.TenantID, pl.meta)
	return err == nil && eff.Decision != mcpserver.OutcomeDeny // hidden only when the tenant default denies
}

func annotations(priority float64) *mcp.Annotations {
	return &mcp.Annotations{Audience: []mcp.Role{"user", "assistant"}, Priority: priority}
}

// ListTemplates is static; templates the tenant policy denies are hidden.
func (r *Provider) ListTemplates(ctx context.Context, p mcpserver.Principal) ([]*mcp.ResourceTemplate, error) {
	out := []*mcp.ResourceTemplate{}
	for _, d := range templateDefs {
		if !r.templateVisible(ctx, p, d) {
			continue
		}
		t := &mcp.ResourceTemplate{Name: d.name, Title: d.title, Description: d.desc, MIMEType: d.mime, URITemplate: d.uri, Annotations: annotations(d.priority)}
		if d.untrusted {
			t.Meta = mcp.Meta{"orca/untrusted": true}
		}
		out = append(out, t)
	}
	return out, nil
}

// ListResources lists orca://projects plus one entry per visible project. A
// failure to read the projects degrades to the root resource only when the
// read was denied; real errors propagate.
func (r *Provider) ListResources(ctx context.Context, p mcpserver.Principal) ([]*mcp.Resource, error) {
	root := &mcp.Resource{URI: "orca://projects", Name: "projects", Title: "Projects", MIMEType: "application/json",
		Description: "Projects visible to you.", Annotations: annotations(0.8)}
	ref, _ := Parse("orca://projects")
	res, err := r.read(ctx, p, "", ref)
	if err != nil {
		if errors.Is(err, mcpserver.ErrResourceNotFound) {
			return []*mcp.Resource{}, nil
		}
		return nil, err
	}
	out := []*mcp.Resource{root}
	for _, e := range projectEntries(res) {
		out = append(out, &mcp.Resource{URI: "orca://project/" + e.id, Name: "project-" + e.id, Title: e.name, MIMEType: "application/json",
			Description: "Project and its worktrees.", Annotations: annotations(0.6)})
	}
	return out, nil
}

type projectEntry struct{ id, name string }

// projectEntries pulls (id, name) pairs from the project.list document
// without depending on its envelope key.
func projectEntries(res *mcp.ReadResourceResult) []projectEntry {
	if res == nil || len(res.Contents) == 0 {
		return nil
	}
	var doc map[string]any
	if json.Unmarshal([]byte(res.Contents[0].Text), &doc) != nil {
		return nil
	}
	var out []projectEntry
	for _, v := range doc {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		for _, it := range arr {
			m, _ := it.(map[string]any)
			id, _ := m["id"].(string)
			if !uuidRe.MatchString(id) {
				continue
			}
			name, _ := m["name"].(string)
			out = append(out, projectEntry{id: strings.ToLower(id), name: truncateName(name)})
		}
	}
	return out
}

func truncateName(s string) string {
	if r := []rune(s); len(r) > 100 {
		return string(r[:100])
	}
	return s
}
