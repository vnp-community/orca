package tools

// pack2SCM: reversible writes to GitHub, GitLab and hosted reviews. Each is
// an open-world call (it reaches the provider) and none is idempotent.
func pack2SCM() []*ToolSpec {
	repo := Str("repo", "repo", "Repository id", Req)
	slug := Str("item_slug", "itemSlug", "Item slug, as returned by github_project_* tools", Req)
	num := Int("number", "number", "Issue or pull request number", Req, Range(1, 1<<30))
	text := func(n, w, d string) Field { return Str(n, w, d, Len(60000)) }
	specs := []*ToolSpec{
		write("github.project.addIssueCommentBySlug", "Add a comment to a GitHub issue or pull request.", slug,
			Str("body", "body", "Comment text", Req, Len(60000))),
		write("github.project.updateIssueCommentBySlug", "Edit a comment on a GitHub issue or pull request.", slug,
			Str("comment_id", "commentId", "Comment id", Req), Str("body", "body", "New text", Req, Len(60000))).idempotent(),
		write("github.project.updateIssueBySlug", "Update a GitHub issue.", slug, text("title", "title", "New title"),
			text("body", "body", "New body"), Str("state", "state", "New state"),
			Strs("add_labels", "addLabels", "Labels to add"), Strs("remove_labels", "removeLabels", "Labels to remove")).idempotent(),
		write("github.project.updateIssueTypeBySlug", "Set the type of a GitHub issue.", slug,
			Str("issue_type", "issueType", "Issue type", Req)).idempotent(),
		write("github.project.updatePullRequestBySlug", "Update a GitHub pull request title, body or state.", slug,
			text("title", "title", "New title"), text("body", "body", "New body"), Str("state", "state", "New state")).idempotent(),
		write("github.project.updateItemField", "Set a field of a GitHub Project item.", Str("project_slug", "projectSlug", "Project slug", Req),
			Str("item_id", "itemId", "Item id", Req), Str("field_id", "fieldId", "Field id", Req),
			Str("kind", "kind", "Field kind", Req), Str("value", "value", "New value", Req)).idempotent(),
		write("github.project.clearItemField", "Clear a field of a GitHub Project item.", Str("project_slug", "projectSlug", "Project slug", Req),
			Str("item_id", "itemId", "Item id", Req), Str("field_id", "fieldId", "Field id", Req)).idempotent(),
		write("github.updateIssue", "Update a GitHub issue.", repo, num, text("title", "title", "New title"),
			text("body", "body", "New body"), Str("state", "state", "New state"),
			Strs("add_labels", "addLabels", "Labels to add"), Strs("remove_labels", "removeLabels", "Labels to remove"),
			Strs("assignees", "assignees", "Assignee logins")).idempotent(),
		write("github.updatePRTitle", "Change the title of a GitHub pull request.", repo,
			Int("pr_number", "prNumber", "Pull request number", Req, Range(1, 1<<30)), Str("title", "title", "New title", Req, Len(500))).idempotent(),
		write("github.requestPRReviewers", "Request reviewers on a GitHub pull request.", repo, num,
			Strs("reviewer_logins", "reviewerLogins", "Reviewer logins"), Strs("team_slugs", "teamSlugs", "Team slugs")).idempotent(),
		write("github.removePRReviewers", "Remove requested reviewers from a GitHub pull request.", repo, num,
			Strs("reviewer_logins", "reviewerLogins", "Reviewer logins", Req)).idempotent(),
		write("gitlab.resolveMRDiscussion", "Resolve or reopen a GitLab merge request discussion.", repo,
			Int("merge_request_iid", "mergeRequestIid", "Merge request number", Req, Range(1, 1<<30)),
			Str("discussion_id", "discussionId", "Discussion id", Req), Bool("resolved", "resolved", "Resolved state", Req)).idempotent(),
		write("hostedReview.create", "Open a pull or merge request. Close it on the provider to undo.",
			Str("provider", "provider", "Hosting provider", Req, OneOf("github", "gitlab")), repo,
			Str("title", "title", "Title", Req, Len(500)), text("body", "body", "Description"),
			Str("head_branch", "headBranch", "Head branch", Req), Str("base_branch", "baseBranch", "Base branch"),
			Str("request_id", "requestId", "Idempotency key"), Bool("draft", "draft", "Open as draft"),
			Int("linked_issue_number", "linkedIssueNumber", "Issue to link", Range(1, 1<<30))),
		write("hostedReview.submit", "Submit unsent annotations as a review.", Str("repo_id", "repoId", "Repository id", Req),
			Str("provider", "provider", "Hosting provider", Req, OneOf("github", "gitlab")),
			Int("pr_number", "prNumber", "Pull request number", Req, Range(1, 1<<30)),
			Str("review_type", "reviewType", "Review type", Req), text("summary", "summary", "Review summary")),
	}
	for _, s := range specs {
		s.openWorld()
	}
	return specs
}

// pack2Trackers: Jira, Linear, workflow templates, automations and projects.
func pack2Trackers() []*ToolSpec {
	site := Str("site_id", "siteId", "Jira site id")
	ws := Str("workspace_id", "workspaceId", "Linear workspace id")
	body := Str("body", "body", "Comment text", Req, Len(60000))
	desc := Str("description", "description", "Description", Len(60000))
	labels := Strs("label_ids", "labelIds", "Label ids")
	specs := []*ToolSpec{
		write("jira.addIssueComment", "Add a comment to a Jira issue.", Str("issue_id_or_key", "issueIdOrKey", "Issue id or key", Req), body, site),
		write("jira.createIssue", "Create a Jira issue.", Str("project_key", "projectKey", "Project key", Req),
			Str("title", "title", "Summary", Req, Len(500)), desc, Str("issue_type", "issueType", "Issue type", Req),
			Str("assignee_id", "assigneeId", "Assignee id"), Str("priority_id", "priorityId", "Priority id"), labels, site),
		write("jira.updateIssue", "Update a Jira issue; a transition changes its status.", Str("issue_id_or_key", "issueIdOrKey", "Issue id or key", Req),
			Str("title", "title", "Summary", Len(500)), desc, Str("assignee_id", "assigneeId", "Assignee id"),
			Str("priority_id", "priorityId", "Priority id"), labels, Str("transition_id", "transitionId", "Transition id"), site),
		write("linear.addIssueComment", "Add a comment to a Linear issue.", Str("issue_id", "issueId", "Issue id", Req), body, ws),
		write("linear.createIssue", "Create a Linear issue.", Str("team_id", "teamId", "Team id", Req),
			Str("title", "title", "Title", Req, Len(500)), desc, Str("state_id", "stateId", "State id"),
			Str("assignee_id", "assigneeId", "Assignee id"), labels, Str("parent_id", "parentId", "Parent issue id"), ws),
		write("linear.updateIssue", "Update a Linear issue.", Str("issue_id", "issueId", "Issue id", Req),
			Str("title", "title", "Title", Len(500)), desc, Str("state_id", "stateId", "State id"),
			Str("assignee_id", "assigneeId", "Assignee id"), labels, ws),
		write("linear.createProject", "Create a Linear project.", Str("team_id", "teamId", "Team id", Req),
			Str("name", "name", "Project name", Req, Len(200)), desc, ws),
	}
	for _, s := range specs {
		s.openWorld()
	}
	return append(specs,
		write("workflow.template.create", "Create a workflow template from a DAG definition.", Str("name", "name", "Template name", Req, Len(200)),
			Str("dag_json", "dagJson", "DAG definition as JSON text", Req, Len(200000)), Str("scope", "scope", "Template scope"),
			Str("parent_template_id", "parentTemplateId", "Parent template id")),
		write("workflow.template.update", "Update a workflow template; pass the version you read.", Str("id", "id", "Template id", Req),
			Str("name", "name", "Template name", Len(200)), Str("dag_json", "dagJson", "DAG definition as JSON text", Len(200000)),
			Str("scope", "scope", "Template scope"), Str("parent_template_id", "parentTemplateId", "Parent template id"),
			Int("expected_version", "expectedVersion", "Version you read", Req, Range(0, 1<<30))),
		write("workflow.template.clone", "Clone a workflow template.", Str("source_template_id", "sourceTemplateId", "Template to clone", Req),
			Str("name", "name", "New name", Req, Len(200)), Str("description", "description", "Description"), Strs("tags", "tags", "Tags")),
		// automation.create is not exposed: it has no way to start disabled,
		// and an enabled schedule would run without review.
		write("automation.update", "Update an automation's name, schedule or step. Enabling is not possible here.",
			Str("id", "id", "Automation id", Req), Str("name", "name", "Name", Len(200)), Str("rrule", "rrule", "Recurrence rule"),
			Str("step_config_json", "stepConfigJson", "Step configuration JSON", Len(200000)), Str("step_type", "stepType", "Step type"),
			Str("dtstart", "dtstart", "Start time"), Str("timezone", "timezone", "Time zone")).idempotent(),
		write("project.create", "Create a project.", Str("name", "name", "Project name", Req, Len(200)), Str("description", "description", "Description", Len(4000)),
			Str("dev_server_id", "devServerId", "Dev server id"), Str("repo_path", "repoPath", "Repository path on the dev server"),
			Str("default_branch", "defaultBranch", "Default branch"), Str("visibility", "visibility", "Visibility")),
		write("project.update", "Update a project.", Str("id", "id", "Project id", Req), Str("name", "name", "Project name", Len(200)),
			Str("description", "description", "Description", Len(4000)), Str("default_branch", "defaultBranch", "Default branch"),
			Str("visibility", "visibility", "Visibility")).idempotent(),
		write("projectGroup.create", "Create a project group.", Str("name", "name", "Group name", Req, Len(200)),
			Str("parent_group_id", "parentGroupId", "Parent group id")),
		write("projectGroup.update", "Rename a project group.", Str("group_id", "groupId", "Group id", Req),
			Str("name", "name", "Group name", Req, Len(200))).idempotent(),
		write("projectGroup.moveProject", "Move a project to another group.", Str("project_id", "projectId", "Project id", Req),
			Str("target_parent_group_id", "targetParentGroupId", "Target group id", Req)).idempotent(),
	)
}
