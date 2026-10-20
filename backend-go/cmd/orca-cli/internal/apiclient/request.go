package apiclient

import (
	"context"
	"net/url"
	"strconv"
)

// One method per api-gateway route of the Request flow (services/api-gateway/.../request_routes*.go). The gateway
// answers with the same views the UI reads, so the paths below are the whole contract between CLI and UI.

func (c *Client) getDoc(ctx context.Context, path string, query url.Values) (Document, error) {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var out Document
	err := c.do(ctx, "GET", path, nil, &out)
	return out, err
}

func (c *Client) postDoc(ctx context.Context, path string, body any) (Document, error) {
	if body == nil {
		body = Document{}
	}
	var out Document
	err := c.do(ctx, "POST", path, body, &out)
	return out, err
}

func (c *Client) putDoc(ctx context.Context, path string, body any) (Document, error) {
	var out Document
	err := c.do(ctx, "PUT", path, body, &out)
	return out, err
}

// GetEngineSettings reads a project's engine switches; check also asks its dev server whether they can run.
func (c *Client) GetEngineSettings(ctx context.Context, projectID string, check bool) (Document, error) {
	q := url.Values{}
	if check {
		q.Set("check", "true")
	}
	return c.getDoc(ctx, "/v1/projects/"+url.PathEscape(projectID)+"/request-engine", q)
}

func (c *Client) SetEngineSettings(ctx context.Context, projectID string, body Document) (Document, error) {
	return c.putDoc(ctx, "/v1/projects/"+url.PathEscape(projectID)+"/request-engine", body)
}

// reviewProfilePath is the drawing profile of a project, or of the tenant when projectID is empty.
func reviewProfilePath(projectID string) string {
	if projectID == "" {
		return "/v1/review-drawing-profile"
	}
	return "/v1/projects/" + url.PathEscape(projectID) + "/review-drawing-profile"
}

// GetReviewProfile reads the drawing profile layers of a project (or the tenant) and the profile in force.
func (c *Client) GetReviewProfile(ctx context.Context, projectID string) (Document, error) {
	return c.getDoc(ctx, reviewProfilePath(projectID), nil)
}

func (c *Client) SaveReviewProfile(ctx context.Context, projectID string, body Document) (Document, error) {
	return c.putDoc(ctx, reviewProfilePath(projectID), body)
}

func (c *Client) ResetReviewProfile(ctx context.Context, projectID string, body Document) (Document, error) {
	return c.postDoc(ctx, reviewProfilePath(projectID)+"/reset", body)
}

func requestPath(id string, rest string) string {
	return "/v1/requests/" + url.PathEscape(id) + rest
}

func (c *Client) CreateRequest(ctx context.Context, body Document) (Document, error) {
	return c.postDoc(ctx, "/v1/requests/", body)
}

func (c *Client) ListRequests(ctx context.Context, q url.Values) (Document, error) {
	return c.getDoc(ctx, "/v1/requests/", q)
}

func (c *Client) GetRequest(ctx context.Context, id string) (Document, error) {
	return c.getDoc(ctx, requestPath(id, ""), nil)
}

func (c *Client) ClassifyRequest(ctx context.Context, id string) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/classify"), nil)
}

func (c *Client) ConfirmRequestType(ctx context.Context, id string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/confirm-type"), body)
}

func (c *Client) CancelRequest(ctx context.Context, id string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/cancel"), body)
}

func (c *Client) ReopenRequest(ctx context.Context, id string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/reopen"), body)
}

func (c *Client) ListSolutions(ctx context.Context, id string, q url.Values) (Document, error) {
	return c.getDoc(ctx, requestPath(id, "/solutions"), q)
}

func (c *Client) GenerateSolution(ctx context.Context, id string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/solutions"), body)
}

func (c *Client) ChooseSolutionOption(ctx context.Context, id, solutionID string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/solutions/"+url.PathEscape(solutionID)+"/choose"), body)
}

func (c *Client) ListRequestApprovals(ctx context.Context, id string, q url.Values) (Document, error) {
	return c.getDoc(ctx, requestPath(id, "/approvals"), q)
}

func (c *Client) ListPendingApprovals(ctx context.Context, q url.Values) (Document, error) {
	return c.getDoc(ctx, "/v1/approvals/pending", q)
}

func (c *Client) GetApproval(ctx context.Context, id string) (Document, error) {
	return c.getDoc(ctx, "/v1/approvals/"+url.PathEscape(id), nil)
}

// DecideApproval approves or rejects (decision is "approve" or "reject") the gate the caller saw at version and digest.
func (c *Client) DecideApproval(ctx context.Context, id, decision string, body Document) (Document, error) {
	return c.postDoc(ctx, "/v1/approvals/"+url.PathEscape(id)+"/"+decision, body)
}

// GeneratePlan is the two-phase plan call: body mode "propose" starts a run, "commit" writes the proposal.
func (c *Client) GeneratePlan(ctx context.Context, id string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/plan"), body)
}

func (c *Client) GetPlanProposal(ctx context.Context, id, runID string) (Document, error) {
	return c.GetPlanProposalRevision(ctx, id, runID, 0)
}

// GetPlanProposalRevision reads one numbered version of the Plan (see ListPlanRevisions); revision 0 reads the run, or the
// current version when runID is empty too.
func (c *Client) GetPlanProposalRevision(ctx context.Context, id, runID string, revision int) (Document, error) {
	q := url.Values{}
	if runID != "" {
		q.Set("runId", runID)
	}
	if revision > 0 {
		q.Set("revision", strconv.Itoa(revision))
	}
	return c.getDoc(ctx, requestPath(id, "/plan"), q)
}

// GetPlanBundleFile reads one file of the authored OpenSpec change behind a Plan (path relative to the change).
func (c *Client) GetPlanBundleFile(ctx context.Context, id, path string) (Document, error) {
	return c.GetPlanBundleFileRevision(ctx, id, path, 0)
}

// GetPlanBundleFileRevision reads the file as it was in a numbered version; revision 0 reads the current one.
func (c *Client) GetPlanBundleFileRevision(ctx context.Context, id, path string, revision int) (Document, error) {
	q := url.Values{"path": {path}}
	if revision > 0 {
		q.Set("revision", strconv.Itoa(revision))
	}
	return c.getDoc(ctx, requestPath(id, "/plan/bundle/file"), q)
}

// ListPlanRevisions lists the versions of the Plan, newest first, with what was asked to get each one.
func (c *Client) ListPlanRevisions(ctx context.Context, id, pageToken string, pageSize int) (Document, error) {
	q := url.Values{}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	if pageSize > 0 {
		q.Set("pageSize", strconv.Itoa(pageSize))
	}
	return c.getDoc(ctx, requestPath(id, "/plan/revisions"), q)
}

// CleanupProposal removes the worktree of the Request's OpenSpec proposal; the branch and the stored change stay.
func (c *Client) CleanupProposal(ctx context.Context, id string) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/proposal/cleanup"), nil)
}

func (c *Client) StartPhase(ctx context.Context, id, phaseTaskID string) (Document, error) {
	return c.postDoc(ctx, requestPath(id, "/phases/"+url.PathEscape(phaseTaskID)+"/start"), nil)
}

// GetTaskExecution reads the newest run's result of one task: status, checks, files, and what the agent reported reading
// in the knowledge graph.
func (c *Client) GetTaskExecution(ctx context.Context, taskID string) (Document, error) {
	return c.getDoc(ctx, "/v1/tasks/"+url.PathEscape(taskID)+"/execution", nil)
}

// GetPlanWritebacks gathers what the agents of a Plan's tasks reported writing back to the design knowledge. Nothing in it
// has been applied by anyone.
func (c *Client) GetPlanWritebacks(ctx context.Context, planTaskID string) (Document, error) {
	return c.getDoc(ctx, "/v1/tasks/"+url.PathEscape(planTaskID)+"/writebacks", nil)
}

func (c *Client) GetTaskSubtree(ctx context.Context, rootTaskID string) (Document, error) {
	return c.getDoc(ctx, "/v1/tasks/"+url.PathEscape(rootTaskID)+"/subtree", nil)
}

func (c *Client) ListClarifications(ctx context.Context, requestID string, q url.Values) (Document, error) {
	return c.getDoc(ctx, requestPath(requestID, "/clarifications"), q)
}

// AskClarification opens a Clarification of the given source about something the Request already holds (the open questions of
// a Solution, for instance). The service decides who may ask and refuses the platform-only sources.
func (c *Client) AskClarification(ctx context.Context, requestID string, body Document) (Document, error) {
	return c.postDoc(ctx, requestPath(requestID, "/clarifications"), body)
}

// ListDecisions reads the decision log of a Request; ConfirmDecision is a person's act on a high risk choice.
func (c *Client) ListDecisions(ctx context.Context, requestID string) (Document, error) {
	return c.getDoc(ctx, requestPath(requestID, "/decisions"), nil)
}

func (c *Client) ConfirmDecision(ctx context.Context, decisionID string, body Document) (Document, error) {
	return c.postDoc(ctx, "/v1/decisions/"+url.PathEscape(decisionID)+"/confirm", body)
}

func (c *Client) AnswerClarification(ctx context.Context, clarificationID string, body Document) (Document, error) {
	return c.postDoc(ctx, "/v1/clarifications/"+url.PathEscape(clarificationID)+"/answer", body)
}

func (c *Client) GetClarification(ctx context.Context, clarificationID string) (Document, error) {
	return c.getDoc(ctx, "/v1/clarifications/"+url.PathEscape(clarificationID), nil)
}
