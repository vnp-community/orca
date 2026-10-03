package tools

// pack3Exec: tools that reach the network or spawn processes. They are
// catalogued but hidden unless pack 3 is enabled (MCP_TOOL_PACKS_ENABLED) and
// the tenant policy allows them. Terminal, agent and workflow-run tools are
// BE-009 and deliberately absent.
func pack3Exec() []*ToolSpec {
	wt := worktreeSel()
	return []*ToolSpec{
		execTool("git.push", "Push the current branch to a remote.", wt, Str("remote", "remote", "Remote name"),
			Str("branch", "branch", "Branch name")).openWorld(),
		execTool("git.pull", "Pull the current branch from its upstream.", wt).openWorld(),
		execTool("git.fetch", "Fetch from the remotes.", wt).openWorld().idempotent(),
		execTool("git.forkSync", "Sync a fork with its upstream.", wt,
			Str("expected_upstream", "expectedUpstream", "Expected upstream repository")).openWorld(),
		execTool("git.fastForward", "Fast-forward the current branch to its upstream.", wt).openWorld(),
		execTool("git.generateCommitMessage", "Draft a commit message from staged changes with the AI provider.", wt).openWorld(),
		execTool("git.generatePullRequestFields", "Draft a pull request title and body with the AI provider.", wt,
			Str("base_branch", "baseBranch", "Base branch")).openWorld(),
		execTool("worktree.prefetchCreateBase", "Fetch the base ref used to create worktrees.", Str("repo_id", "repoId", "Repository id", Req),
			Str("base_ref", "baseRef", "Base ref")).openWorld().idempotent(),
		execTool("repo.clone", "Clone a repository onto a dev server.", Str("dev_server_id", "devServerId", "Dev server id", Req),
			Str("url", "url", "Remote URL", Req), Str("dest_path", "destPath", "Destination path", Req)).openWorld(),
		execTool("automation.runNow", "Run an automation now.", Str("automation_id", "automationId", "Automation id", Req),
			Str("request_id", "requestId", "Idempotency key")).openWorld(),
		execTool("task.execute", "Start the agent workflow of a task.", Str("task_id", "taskId", "Task id", Req),
			Str("request_id", "requestId", "Idempotency key"), Str("prompt", "prompt", "Extra prompt", Len(8000))).openWorld(),
		execTool("annotation.sendToAgent", "Send unsent annotations of a worktree to its agent terminal.", worktreeIDField(),
			Str("worktree_name", "worktreeName", "Worktree display name")),
		execTool("ephemeralVm.attachWorkspace", "Attach a workspace to an ephemeral VM runtime.", Str("runtime_id", "runtimeId", "Runtime id", Req),
			Str("workspace_id", "workspaceId", "Workspace id", Req)).openWorld(),
		execTool("ephemeralVm.resumeWorkspace", "Resume a suspended ephemeral VM workspace.", Str("workspace_id", "workspaceId", "Workspace id", Req)).openWorld(),
		execTool("ephemeralVm.suspendWorkspace", "Suspend an ephemeral VM workspace.", Str("workspace_id", "workspaceId", "Workspace id", Req)).openWorld(),
		execTool("workspacePorts.scan", "Scan listening ports of a workspace.", Str("connection_id", "connectionId", "Connection id"),
			Str("worktree_id", "worktreeId", "Worktree id"), Str("repo_id", "repoId", "Repository id")),
		execTool("workspacePorts.kill", "Kill the process listening on a port.", Str("connection_id", "connectionId", "Connection id"),
			Str("worktree_id", "worktreeId", "Worktree id"), Str("repo_id", "repoId", "Repository id"),
			Int("pid", "pid", "Process id", Req, Range(1, 1<<30)), Int("port", "port", "Port", Req, Range(1, 65535))),
	}
}

// pack4Admin: destructive and administrative tools; off by default, and every
// call needs approval once governance enables them.
func pack4Admin() []*ToolSpec {
	wt := worktreeSel()
	role := Str("role", "role", "Member role", Req)
	// Target of the operation, not the caller: named member_user_id so no input
	// is ever called user_id (the caller identity comes from the token only).
	user := Str("member_user_id", "userId", "Id of the user being changed", Req)
	pageSize := Int("page_size", "pageSize", "Items per page", Range(1, 200))
	pageTok := Str("page_token", "pageToken", "Token from a previous page")
	return []*ToolSpec{
		destructive("worktree.rm", "Delete a worktree and its directory.", Str("worktree_id", "worktree", "Worktree id", Req, IDSel),
			Bool("force", "force", "Delete despite uncommitted work"), Bool("allow_open_pr", "allowOpenPr", "Delete despite an open pull request"),
			Bool("stop_agents", "stopAgents", "Stop agents running in it")),
		destructive("worktree.forceDeleteBranch", "Force-delete the branch of a worktree.", Str("worktree_id", "worktree", "Worktree id", Req, IDSel),
			Str("branch_name", "branchName", "Branch name", Req), Str("expected_head", "expectedHead", "Expected head commit", Req)),
		destructive("worktree.merge", "Merge a worktree into a base branch.", worktreeIDField(), Str("base_branch", "baseBranch", "Base branch", Req),
			Str("strategy", "strategy", "Merge strategy"), Str("commit_message", "commitMessage", "Commit message"),
			Strs("cleanup_worktree_ids", "cleanupWorktreeIds", "Worktrees to delete afterwards")),
		destructive("git.branch.delete", "Delete a local branch.", wt, Str("branch", "branch", "Branch name", Req)),
		destructive("git.discard", "Discard working tree changes of a file.", wt, Str("path", "path", "File path", Req)),
		destructive("git.bulkDiscard", "Discard working tree changes of many files.", wt, Strs("paths", "paths", "File paths", Req)),
		destructive("repo.rm", "Remove a repository from Orca.", Str("repo_id", "repoId", "Repository id", Req)),
		destructive("project.delete", "Delete a project.", Str("project_id", "projectId", "Project id", Req)),
		destructive("automation.delete", "Delete an automation.", Str("id", "id", "Automation id", Req)),
		destructive("task.delete", "Delete a task.", Str("id", "id", "Task id", Req)),
		// confirmed is the handler's own safeguard: the agent must state it explicitly.
		destructive("annotation.delete", "Delete a code review annotation.", Str("id", "id", "Annotation id", Req),
			Bool("confirmed", "confirmed", "Must be true to delete", Req)),
		destructive("projectGroup.delete", "Delete a project group.", Str("group_id", "groupId", "Project group id", Req)),
		destructive("github.project.deleteIssueCommentBySlug", "Delete a comment of a GitHub issue or pull request.",
			Str("item_slug", "itemSlug", "Item slug, as returned by github_project_* tools", Req),
			Str("comment_id", "commentId", "Comment id", Req)).openWorld(),
		destructive("github.mergePR", "Merge a GitHub pull request.", Str("repo", "repo", "Repository id", Req),
			Int("number", "number", "Pull request number", Req, Range(1, 1<<30)), Str("merge_method", "mergeMethod", "merge, squash or rebase"),
			Str("commit_title", "commitTitle", "Commit title"), Str("commit_message", "commitMessage", "Commit message")).openWorld(),
		destructive("github.setPRAutoMerge", "Enable or disable auto-merge on a GitHub pull request.", Str("repo", "repo", "Repository id", Req),
			Int("number", "number", "Pull request number", Req, Range(1, 1<<30)), Bool("enabled", "enabled", "Auto-merge on", Req),
			Str("merge_method", "mergeMethod", "merge, squash or rebase")).openWorld(),
		destructive("ephemeralVm.cleanup", "Delete an ephemeral VM runtime.", Str("runtime_id", "runtimeId", "Runtime id", Req)).openWorld(),
		destructive("project.addMember", "Add a user to a project.", Str("project_id", "projectId", "Project id", Req), user, role),
		destructive("project.removeMember", "Remove a user from a project.", Str("project_id", "projectId", "Project id", Req), user),
		destructive("project.updateMemberRole", "Change a project member's role.", Str("project_id", "projectId", "Project id", Req), user, role),
		destructive("repo.addMember", "Add a user to a repository.", Str("repo_id", "repoId", "Repository id", Req), user, role),
		destructive("repo.removeMember", "Remove a user from a repository.", Str("repo_id", "repoId", "Repository id", Req), user),
		destructive("repo.updateMemberRole", "Change a repository member's role.", Str("repo_id", "repoId", "Repository id", Req), user, role),
		destructive("devServer.approve", "Approve a dev server.", Str("dev_server_id", "devServerId", "Dev server id", Req)),
		destructive("devServer.reject", "Reject a dev server.", Str("dev_server_id", "devServerId", "Dev server id", Req),
			Str("reason", "reason", "Reason")),
		destructive("devServer.assignGroup", "Assign a dev server to a group.", Str("dev_server_id", "devServerId", "Dev server id", Req),
			Str("group_id", "groupId", "Group id", Req)),
		destructive("devServer.resolveAccessRequest", "Approve or reject a dev server access request.",
			Str("request_id", "requestId", "Request id", Req), Bool("approve", "approve", "Approve the request", Req)),

		adminRead("admin.listUsers", "List users of the tenant.", pageSize, pageTok).pii(),
		adminRead("admin.listSessions", "List login sessions of a user.", Str("target_user_id", "userId", "Id of the user to inspect", Req)).pii(),
		adminRead("admin.queryAuditLog", "Query the audit log.", Int("since_unix_ms", "sinceUnixMs", "Only entries after this time", Range(0, 1<<53)),
			Str("actor_id", "actorId", "Actor id"), Str("action", "action", "Action"), Str("outcome", "outcome", "Outcome"), pageSize, pageTok),
	}
}
