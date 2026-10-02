package tools

import (
	"encoding/json"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// pack2Workspace: reversible writes to tasks, annotations and worktrees.
func pack2Workspace() []*ToolSpec {
	taskID := Str("task_id", "taskId", "Task id", Req)
	return []*ToolSpec{
		write("task.create", "Create a task.", Str("title", "title", "Task title", Req, Len(500)),
			Str("parent_id", "parentId", "Parent task id"), Str("project_id", "projectId", "Project id")),
		write("task.update", "Update a task. Set status back to undo a status change.", Str("id", "id", "Task id", Req),
			Str("title", "title", "New title", Len(500)), Str("status", "status", "New status"),
			Str("workflow_template_id", "workflowTemplateId", "Workflow template id"), Strs("labels", "labels", "Replace labels")).idempotent(),
		write("task.addComment", "Add a comment to a task.", taskID, Str("content", "content", "Comment text", Req, Len(8000))),
		write("task.addEdge", "Add a dependency edge between two tasks.", Str("from_task_id", "fromTaskId", "Source task id", Req),
			Str("to_task_id", "toTaskId", "Target task id", Req), Str("type", "type", "Edge type", Req)).idempotent(),
		write("task.createFromSource", "Create a task from an external issue.", Str("title", "title", "Task title", Req, Len(500)),
			Str("provider", "provider", "Issue provider", Req), Str("ref", "ref", "Issue reference", Req),
			Str("url", "url", "Issue web address"), Str("parent_id", "parentId", "Parent task id"), Str("project_id", "projectId", "Project id")),
		write("task.aiDecompose", "Ask the AI provider to propose subtasks for a task. Uses provider quota.", taskID).openWorld(),
		write("task.aiApply", "Apply AI subtask proposals.", taskID).declared(),
		write("task.recalculateProgress", "Recalculate progress of a task tree.", Str("root_id", "rootId", "Root task id", Req)).idempotent(),

		annotationCreate(),
		write("annotation.update", "Edit or resolve an annotation.", Str("id", "id", "Annotation id", Req),
			Str("content", "content", "Annotation text", Req, Len(8000)), Bool("resolved", "resolved", "Mark resolved", Req)).idempotent(),
		write("annotation.markSent", "Mark annotations as sent to an agent.", Strs("ids", "ids", "Annotation ids", Req)).idempotent(),

		worktreeCreate(),
		write("worktree.createFromIssue", "Create a worktree from an issue of a provider.", Str("project_id", "projectId", "Project id", Req),
			Str("repo_id", "repoId", "Repository id", Req), Str("base_ref", "baseRef", "Base ref"),
			Str("provider", "provider", "Issue provider", Req, OneOf("github", "gitlab", "jira", "linear")),
			Str("repo", "repo", "Repository for github or gitlab"), Int("number", "number", "Issue number for github or gitlab", Range(1, 1<<30)),
			Str("issue_ref", "issueRef", "Issue key for jira or linear")).consts(map[string]any{"skipAgentStart": true}).openWorld(),
		write("worktree.set", "Activate or deactivate a worktree, or set its parent.", worktreeSel(),
			Bool("active", "active", "Activation state"), Str("parent_worktree_id", "parentWorktree", "Parent worktree id", IDSel),
			Bool("no_parent", "noParent", "Clear the parent")).idempotent(),
		write("worktree.fanOut", "Create up to 5 sibling worktrees from one base ref, each with an agent prompt.",
			Str("project_id", "projectId", "Project id", Req), Str("repo_id", "repoId", "Repository id", Req),
			Str("base_ref", "baseRef", "Base ref"), Str("branch_prefix", "branchPrefix", "Branch name prefix", Req),
			Str("prompt", "prompt", "Prompt for each agent", Len(8000)), Str("agent_type", "agentType", "Agent type"),
			Int("n", "n", "Number of worktrees", Req, Range(1, 5))).openWorld(),
	}
}

// Git writes use the same "worktree" selector as the reads. Each stays
// reversible through reflog, stash or the abort* counterparts.
func pack2Git() []*ToolSpec {
	wt := worktreeSel()
	paths := Strs("paths", "paths", "File paths", Req)
	return []*ToolSpec{
		write("git.stage", "Stage files.", wt, paths).idempotent(),
		write("git.unstage", "Unstage files.", wt, paths).idempotent(),
		write("git.bulkStage", "Stage many files.", wt, paths).idempotent(),
		write("git.bulkUnstage", "Unstage many files.", wt, paths).idempotent(),
		write("git.commit", "Commit staged changes (or the given paths). Undo with git reflog.", wt,
			Str("message", "message", "Commit message", Req, Len(4000)), Strs("paths", "paths", "Limit to these paths")),
		write("git.checkout", "Check out a branch.", wt, Str("branch", "branch", "Branch name", Req)),
		write("git.branch.create", "Create a branch, optionally checking it out.", wt, Str("branch", "branch", "Branch name", Req),
			Str("base_ref", "baseRef", "Start point"), Bool("checkout", "checkout", "Check out after creating")),
		write("git.stash.push", "Stash local changes.", wt, Str("message", "message", "Stash message"),
			Bool("include_untracked", "includeUntracked", "Include untracked files")),
		write("git.stash.pop", "Apply and drop a stash entry.", wt, Str("stash_ref", "stashRef", "Stash reference", Req)),
		write("git.merge", "Merge a branch into the worktree branch. Use git_abortMerge to back out.", wt,
			Str("branch", "branch", "Branch to merge", Req), Bool("no_ff", "noFf", "Always create a merge commit")),
		write("git.rebaseFromBase", "Rebase onto a base branch. Use git_abortRebase to back out.", wt,
			Str("base_branch", "baseBranch", "Base branch", Req)),
		write("git.abortMerge", "Abort a merge in progress.", wt).idempotent(),
		write("git.abortRebase", "Abort a rebase in progress.", wt).idempotent(),
		write("git.resolveConflict", "Resolve one conflicted file.", wt, Str("path", "path", "File path", Req),
			Str("operation", "operation", "Resolution to apply", Req)),
	}
}

func annotationCreate() *ToolSpec {
	s := write("annotation.create", "Create a code review annotation on a line.",
		Str("repo_id", "repoId", "Repository id", Req), Str("worktree_id", "worktreeId", "Worktree id"),
		Str("file_path", "filePath", "File path", Req), Int("line", "line", "Line number", Req, Range(1, 1<<30)),
		Int("end_line", "endLine", "End line for a range", Range(1, 1<<30)), Int("side", "side", "Diff side", Range(0, 2)),
		Str("ref", "ref", "Git ref"), Str("content", "content", "Annotation text", Req, Len(8000)),
		Str("original_code", "originalCode", "Code under the annotation", Len(8000)),
		Str("request_id", "requestId", "Idempotency key"))
	fields := s.Fields
	s.ArgsOverride = func(in json.RawMessage, _ wscompat.Identity) ([]json.RawMessage, error) {
		flat, err := defaultArgs(fields, nil, in)
		if err != nil {
			return nil, err
		}
		var m map[string]json.RawMessage
		_ = json.Unmarshal(flat[0], &m)
		anchor := map[string]json.RawMessage{}
		for _, k := range []string{"repoId", "worktreeId", "filePath", "line", "endLine", "side", "ref"} {
			if v, ok := m[k]; ok {
				anchor[k] = v
				delete(m, k)
			}
		}
		ab, _ := json.Marshal(anchor)
		m["anchor"] = ab
		out, err := json.Marshal(m)
		return []json.RawMessage{out}, err
	}
	return s
}

func worktreeCreate() *ToolSpec {
	// Handler keys are repo/name/baseBranch (not repoId/branch/baseRef); the
	// origin markers make agent-created worktrees visible as such in the UI.
	return write("worktree.create", "Create a worktree. Remove it later with the destructive worktree tool.",
		Str("repo_id", "repo", "Repository id (see repo_list)", Req), Str("name", "name", "Worktree name", Req, Len(200)),
		Str("base_branch", "baseBranch", "Base branch"), Str("display_name", "displayName", "Display name", Len(200)),
		Str("branch_name", "branchNameOverride", "Branch name override", Len(200)),
	).consts(map[string]any{"origin": "mcp", "captureSource": "mcp"})
}
