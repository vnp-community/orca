package tools

// pack1Workspace: projects, repos, worktrees, tasks, annotations.
func pack1Workspace() []*ToolSpec {
	projectID := Str("project_id", "projectId", "Project id (see project_list)", Req)
	repoID := Str("repo_id", "repoId", "Repository id (see repo_list)", Req)
	taskID := Str("task_id", "taskId", "Task id (see task_list)", Req)
	return []*ToolSpec{
		read("project.list", "List projects of the tenant.", pageFields()...).asList(),
		read("project.get", "Get one project.", projectID),
		read("project.getMembers", "List members of a project.", projectID).asList(),
		read("projectGroup.list", "List project groups.").asList(),
		read("orcaProjects.list", "List shared Orca projects with their source projects.").asList(),
		read("orcaProjects.getProjectData", "Get repos and worktrees a source project shares into an Orca project.",
			Str("orca_project_id", "orcaProjectId", "Orca project id", Req), projectID),
		read("repo.list", "List repositories of a project.", projectID),
		read("repo.getMembers", "List members of a repository.", repoID).asList(),
		read("repo.baseRefDefault", "Get the default base ref of a repository.", repoID),
		read("repo.searchRefs", "Search branches and tags of a repository.", repoID,
			Str("query", "query", "Text to match", Req, Len(200))),

		read("worktree.list", "List worktrees of a project.", projectID),
		read("worktree.detectedList", "List worktrees detected on disk for a repository, with ownership.",
			projectID, repoID),
		read("worktree.lineageList", "List worktree lineage (parent and origin) records.").asList(),
		read("worktree.checkDeleteSafety", "Check whether deleting a worktree would lose work.", worktreeIDField()),
		read("worktree.compare", "Compare several worktrees.",
			Strs("worktree_ids", "worktreeIds", "Worktree ids to compare", Req)),

		read("task.list", "List tasks.", with(pageFields(), Str("project_id", "projectId", "Filter by project id"))...),
		read("task.get", "Get one task.", Str("id", "id", "Task id", Req)),
		read("task.getSubtree", "Get a task and its descendants.", Str("root_id", "rootId", "Root task id", Req)),
		read("task.getDependencies", "Get dependency edges of a task.", taskID),
		read("task.listComments", "List comments of a task.", with(pageFields(), taskID)...).untrusted(),
		read("task.hasActiveExecutions", "Tell whether a project has running task executions.", projectID),
		read("task.getSource", "Get the external issue a task was created from.", taskID),
		read("annotation.list", "List code review annotations.", with(pageFields(),
			Str("repo_id", "repoId", "Filter by repository id"),
			Str("worktree_id", "worktreeId", "Filter by worktree id"),
			Str("file_path", "filePath", "Filter by file path"))...).untrusted(),
		read("annotation.composeReviewPrompt", "Compose a review prompt from the unsent annotations of a worktree.",
			worktreeIDField(), Str("worktree_name", "worktreeName", "Worktree display name")),
	}
}

// pack1Git: read-only git inspection. Wire key is "worktree" with an "id:"
// selector prefix (not worktreeId) in every git.* channel.
func pack1Git() []*ToolSpec {
	wt := worktreeSel()
	base := Str("base_ref", "baseRef", "Base ref to compare against", Req)
	return []*ToolSpec{
		read("git.status", "Get the git status of a worktree.", wt),
		read("git.diff", "Get the diff of a worktree, optionally for one file or the staged changes.", wt,
			Str("file_path", "filePath", "Limit to this path"), Bool("staged", "staged", "Diff the index instead of the working tree")).untrusted(),
		read("git.history", "List recent commits.", wt, Str("base_ref", "baseRef", "Only commits after this ref"),
			Int("limit", "limit", "Max commits", Range(1, 200))),
		read("git.localBranches", "List local branches.", wt).asList(),
		read("git.branchCompare", "Summarize changes between the worktree branch and a base ref.", wt, base),
		read("git.branchDiff", "Diff one file between the worktree branch and a base ref.", wt, base,
			Str("file_path", "filePath", "File path", Req), Str("old_path", "oldPath", "Previous path when renamed")).untrusted(),
		read("git.commitCompare", "List files changed by a commit.", wt, Str("commit_id", "commitId", "Commit id", Req)),
		read("git.commitDiff", "Diff one file of a commit.", wt, Str("commit_oid", "commitOid", "Commit id", Req),
			Str("parent_oid", "parentOid", "Parent commit id"), Str("file_path", "filePath", "File path", Req),
			Str("old_path", "oldPath", "Previous path when renamed")).untrusted(),
		read("git.checkIgnored", "Tell which paths are git-ignored.", wt, Strs("paths", "paths", "Paths to check", Req)),
		read("git.submoduleStatus", "Get the status of a submodule.", wt,
			Str("submodule_path", "submodulePath", "Submodule path", Req), Str("area", "area", "Status area")),
		read("git.upstreamStatus", "Get ahead/behind counts against the upstream.", wt),
		read("git.conflictOperation", "Tell whether a merge or rebase is in progress.", wt),
		read("git.remoteCommitUrl", "Get the web URL of a commit on the remote.", wt, Str("sha", "sha", "Commit id", Req)),
		read("git.remoteFileUrl", "Get the web URL of a file on the remote.", wt, Str("path", "path", "File path", Req),
			Str("ref", "ref", "Ref to link to")),
	}
}
