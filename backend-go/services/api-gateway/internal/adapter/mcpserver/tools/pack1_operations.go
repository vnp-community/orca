package tools

// pack1Operations: workflows, automations, files, infrastructure and tenant
// directory reads.
func pack1Operations() []*ToolSpec {
	wt := worktreeIDField()
	path := Str("path", "path", "Path relative to the worktree root", Req)
	return []*ToolSpec{
		read("workflow.listExecutions", "List workflow executions of a project.",
			Str("project_id", "projectId", "Project id", Req), Int("limit", "limit", "Max items", Range(1, 100)),
			Str("cursor", "cursor", "Cursor from a previous page")),
		read("workflow.getExecution", "Get one workflow execution.", Str("execution_id", "executionId", "Execution id", Req)),
		read("terminal.list", "List open terminals; those created by an MCP client carry an origin.",
			Str("connection_id", "connectionId", "Only terminals of this connection")).asList(),
		read("workflow.hasActiveExecutions", "Tell whether a project has running workflows.", Str("project_id", "projectId", "Project id", Req)),
		read("workflow.template.list", "List workflow templates.", with(pageFields(), Str("scope", "scope", "Template scope"))...),
		read("workflow.template.resolve", "Resolve a workflow template with its parents.", Str("template_id", "templateId", "Template id", Req)),
		read("automation.list", "List automations.", pageFields()...),
		read("automation.runs", "List runs of an automation.", with(pageFields(), Str("automation_id", "automationId", "Automation id", Req))...),

		read("files.readChunk", "Read a byte range of a file; content is base64.", wt, path,
			Int("offset_bytes", "offsetBytes", "Start offset", Range(0, 1<<40)),
			Int("length_bytes", "lengthBytes", "Bytes to read", Range(1, 65536))).untrusted(),
		read("files.readPreview", "Read a UTF-8 text file (first 64 KiB). Binary files return metadata only.", wt, path,
			Int("max_bytes", "maxBytes", "Max bytes to read", Range(1, 65536))).named("files_read", "text-decoded preview; raw base64 is never returned").untrusted().postWith(decodeFilePreview),
		read("files.readDir", "List a directory of a worktree.", wt, Str("path", "path", "Directory path; empty for the root")).asList(),
		read("files.stat", "Get file metadata.", wt, path),
		read("files.search", "Search file contents in a worktree.", wt, Str("pattern", "pattern", "Search pattern", Req, Len(500)),
			Bool("is_regex", "isRegex", "Treat pattern as a regular expression"), Str("path_glob", "pathGlob", "Only paths matching this glob"),
			Int("max_results", "maxResults", "Max matches", Range(1, 500))).untrusted(),
		read("files.listAll", "List files of a worktree.", wt, Str("path_glob", "pathGlob", "Only paths matching this glob"),
			Int("max_results", "maxResults", "Max files", Range(1, 5000))),
		read("files.listMarkdownDocuments", "List Markdown documents of a worktree.", wt,
			Int("max_results", "maxResults", "Max files", Range(1, 5000))),

		read("aiProvider.list", "List configured AI providers (no credentials)."),
		read("devServer.list", "List dev servers.").asList(),
		read("devServer.listForUser", "List dev servers the user can access.").asList(),
		read("ephemeralVm.listRecipes", "List ephemeral VM recipes of a repository.", Str("repo_id", "repoId", "Repository id", Req)),
		read("ephemeralVm.listRecipeCatalog", "List ephemeral VM recipes across projects."),
		read("ephemeralVm.listRuntimes", "List ephemeral VM runtimes."),
		read("agentSession.listActive", "List agent sessions dispatched for the user.").asList(),
		read("team.list", "List teams.").asList(),
		read("team.listMembers", "List members of a team.", Str("team_id", "teamId", "Team id", Req)).asList(),
		read("auth.listTenantMemberDirectory", "List tenant members (names and ids).").asList(),
		read("fleet.health.checkAll", "Check health of the dev server fleet.", Strs("server_ids", "serverIds", "Limit to these dev server ids")),
		read("connectivity.getSummary", "Summarize connectivity of the dev server fleet."),
	}
}

func (s *ToolSpec) postWith(f func(any) any) *ToolSpec { s.Post = f; return s }
