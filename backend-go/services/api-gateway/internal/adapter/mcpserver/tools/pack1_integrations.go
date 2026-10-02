package tools

// pack1SCM: GitHub, GitLab and provider-neutral hosted review reads.
func pack1SCM() []*ToolSpec {
	repo := Str("repo", "repo", "Repository id", Req)
	slug := Str("item_slug", "itemSlug", "Item slug, as returned by github_project_* tools", Req)
	provider := Str("provider", "provider", "Hosting provider", Req, OneOf("github", "gitlab"))
	issueFilters := []Field{repo, Str("state", "state", "Issue state, e.g. open or closed"),
		Str("assignee", "assignee", "Assignee login"), Strs("labels", "labels", "Label names"),
		Str("milestone", "milestone", "Milestone")}
	return []*ToolSpec{
		read("github.issues", "List GitHub issues of a repository.", issueFilters...).scm().asList(),
		read("github.listWorkItems", "List GitHub issues and pull requests of a repository.", repo,
			Int("limit", "limit", "Max items", Range(1, 100)), Str("query", "query", "Search text"),
			Str("before", "before", "Cursor from a previous page")).scm(),
		read("github.prForBranch", "Find the GitHub pull request of a branch.", repo,
			Str("head_branch", "headBranch", "Head branch", Req)).scm(),
		read("github.issueComments", "List comments of a GitHub issue or pull request.", slug).scm().asList(),
		read("github.repoSlug", "Resolve a repository slug from a remote or name.",
			Str("candidate", "candidate", "Remote URL or name", Req)).scm(),
		read("github.rateLimit", "Show the GitHub API rate limit status.").openWorld(),
		read("github.project.listAccessible", "List GitHub Projects the user can access.").scm().asList(),
		read("github.project.listViews", "List views of a GitHub Project.",
			Str("project_slug", "projectSlug", "Project slug", Req)).scm().asList(),
		read("github.project.viewTable", "Read the table of a GitHub Project view.",
			Str("project_slug", "projectSlug", "Project slug", Req), Str("view_id", "viewId", "View id", Req),
			Str("page_token", "pageToken", "Token from a previous page"),
			Int("page_size", "pageSize", "Rows per page", Range(1, 100))).scm(),
		read("github.project.workItemDetailsBySlug", "Get details of a GitHub issue or pull request.", slug).scm(),
		read("github.project.listLabelsBySlug", "List labels available for an item's repository.", slug).scm().asList(),
		read("github.project.listIssueTypesBySlug", "List issue types available for an item's repository.", slug).scm().asList(),
		read("github.project.listAssignableUsersBySlug", "List users assignable to an item.", slug).scm().asList(),

		read("gitlab.issues", "List GitLab issues of a repository.", issueFilters...).scm().asList(),
		read("gitlab.listMRs", "List GitLab merge requests.", repo,
			Str("state", "state", "Merge request state, e.g. opened or merged"),
			Str("source_branch", "sourceBranch", "Filter by source branch")).scm().asList(),
		read("gitlab.workItemDetails", "Get details of a GitLab issue or merge request.", repo,
			Int("iid", "iid", "Project-scoped number", Req, Range(1, 1<<30)),
			Str("item_type", "itemType", "Item type, issue or merge_request")).scm(),
		read("gitlab.rateLimit", "Show the GitLab API rate limit status.").openWorld(),

		read("hostedReview.forBranch", "Find the pull or merge request of a branch.", provider, repo,
			Str("head_branch", "headBranch", "Head branch", Req)).scm(),
		read("hostedReview.getCreationEligibility", "Check whether a pull or merge request can be created.", provider, repo,
			Str("head_branch", "headBranch", "Head branch", Req), Str("base_branch", "baseBranch", "Base branch")).scm(),
		read("hostedReview.suggestReviewers", "Suggest reviewers for changed files.", provider, repo,
			Str("base_ref", "baseRef", "Base ref"), Strs("changed_files", "changedFiles", "Changed file paths")).scm(),
	}
}

// pack1Trackers: Jira and Linear reads. Ticket text is third-party content,
// so results carry untrusted=true.
func pack1Trackers() []*ToolSpec {
	site := Str("site_id", "siteId", "Jira site id")
	ws := Str("workspace_id", "workspaceId", "Linear workspace id")
	issue := Str("issue_id_or_key", "issueIdOrKey", "Issue id or key", Req)
	project := Str("project_id_or_key", "projectIdOrKey", "Project id or key", Req)
	lim := Int("limit", "limit", "Max items", Range(1, 100))
	team := Str("team_id", "teamId", "Team id", Req)
	return []*ToolSpec{
		read("jira.status", "Show the Jira connection status."),
		read("jira.listProjects", "List Jira projects.", site).asList(),
		read("jira.listIssues", "List issues of a Jira project.", Str("project_key", "projectKey", "Project key", Req), lim, site).untrusted().asList(),
		read("jira.searchIssues", "Search Jira issues with JQL.", Str("jql", "jql", "JQL query", Req, Len(2000)), lim, site).untrusted().asList(),
		read("jira.getIssue", "Get a Jira issue.", issue, site).untrusted(),
		read("jira.issueComments", "List comments of a Jira issue.", issue, site).untrusted().asList(),
		read("jira.listIssueTypes", "List issue types of a Jira project.", project, site).asList(),
		read("jira.listTransitions", "List workflow transitions available for an issue.", issue, site).asList(),
		read("jira.listPriorities", "List Jira priorities.", site).asList(),
		read("jira.listAssignableUsers", "List users assignable in a project or issue.",
			Str("project_id_or_key", "projectIdOrKey", "Project id or key"), Str("issue_id_or_key", "issueIdOrKey", "Issue id or key"), site).asList(),
		read("jira.listCreateFields", "List fields required to create an issue.", project,
			Str("issue_type_id", "issueTypeId", "Issue type id", Req), site),
		read("jira.getProjectStatusOrder", "Get the status order of a Jira project.", project, site),

		read("linear.status", "Show the Linear connection status."),
		read("linear.listTeams", "List Linear teams.", ws).asList(),
		read("linear.listIssues", "List issues of a Linear team.", Str("team_key", "teamKey", "Team key", Req), lim, ws).untrusted().asList(),
		read("linear.searchIssues", "Search Linear issues.", Str("query", "query", "Search text", Req, Len(500)), lim, ws).untrusted().asList(),
		read("linear.getIssue", "Get a Linear issue.", Str("issue_id", "issueId", "Issue id", Req), ws).untrusted(),
		read("linear.issueComments", "List comments of a Linear issue.", Str("issue_id", "issueId", "Issue id", Req), ws).untrusted().asList(),
		read("linear.getProject", "Get a Linear project.", Str("project_id", "projectId", "Project id", Req), ws).untrusted(),
		read("linear.getCustomView", "Get a Linear custom view.", Str("view_id", "viewId", "View id", Req),
			Str("model", "model", "View model"), ws).untrusted(),
		read("linear.teamLabels", "List labels of a Linear team.", team, ws).asList(),
		read("linear.teamMembers", "List members of a Linear team.", team, ws).asList(),
		read("linear.teamStates", "List workflow states of a Linear team.", team, ws).asList(),
	}
}
