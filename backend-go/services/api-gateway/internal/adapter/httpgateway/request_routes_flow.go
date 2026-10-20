package httpgateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// requestFlowRoute maps one REST route onto the channel the UI calls for the same step, so a CLI can drive a
// Request from creation to execution without the browser. URL parameters become channel arguments (they win over
// the body, so a body cannot aim a route at another record).
type requestFlowRoute struct {
	method, pattern, channel string
	params                   map[string]string // URL parameter -> channel argument name
}

var (
	byID       = map[string]string{"id": "id"}
	byRequest  = map[string]string{"id": "requestId"}
	phaseStart = map[string]string{"id": "id", "phaseTaskId": "phaseTaskId"}
	chooseOpt  = map[string]string{"id": "requestId", "solutionId": "solutionId"}
)

// requestFlowRoutes is relative to /v1/requests.
var requestFlowRoutes = []requestFlowRoute{
	{http.MethodPost, "/{id}/classify", "request.classify", byID},
	{http.MethodPost, "/{id}/confirm-type", "request.confirmType", byID},
	{http.MethodPost, "/{id}/change-type", "request.changeType", byID},
	{http.MethodPost, "/{id}/return-to-backlog", "request.returnToBacklog", byID},
	{http.MethodPost, "/{id}/reopen", "request.reopen", byID},
	{http.MethodPost, "/{id}/cancel", "request.cancel", byID},
	{http.MethodGet, "/{id}/solutions", "solution.list", byRequest},
	{http.MethodPost, "/{id}/solutions", "solution.generate", byRequest},
	{http.MethodPost, "/{id}/solutions/{solutionId}/choose", "solution.choose", chooseOpt},
	{http.MethodGet, "/{id}/approvals", "approval.list", byRequest},
	{http.MethodGet, "/{id}/plan", "request.planProposal", byID},
	{http.MethodPost, "/{id}/plan", "request.generatePlan", byID},
	{http.MethodGet, "/{id}/plan/bundle", "request.planProposal", byID}, // the proposal view carries the bundle listing, checks and knowledge
	{http.MethodGet, "/{id}/plan/bundle/file", "request.planBundleFile", byID},
	{http.MethodGet, "/{id}/plan/revisions", "request.planRevisions", byID}, // the versions of the Plan, newest first (CR-REQ-045)
	{http.MethodPost, "/{id}/proposal/cleanup", "request.cleanupProposal", byID},
	{http.MethodPost, "/{id}/phases/{phaseTaskId}/start", "request.startPhase", phaseStart},
	{http.MethodGet, "/{id}/clarifications", "clarification.list", byRequest},
	{http.MethodPost, "/{id}/clarifications", "clarification.request", byRequest},
	{http.MethodGet, "/{id}/decisions", "decision.list", byRequest},
}

// projectEngineRoutes are absolute paths: ?check=true also asks the dev server whether what is switched on can run.
var projectEngineRoutes = []requestFlowRoute{
	{http.MethodGet, "/v1/projects/{id}/request-engine", "request.engineSettings", map[string]string{"id": "projectId"}},
	{http.MethodPut, "/v1/projects/{id}/request-engine", "request.engineSettingsSet", map[string]string{"id": "projectId"}},
}

// approvalFlowRoutes is relative to /v1/approvals: a decider reads the version and digest it is about to decide.
var approvalFlowRoutes = []requestFlowRoute{
	{http.MethodGet, "/{id}", "approval.get", byID},
}

// decisionFlowRoutes is relative to /v1/decisions.
var decisionFlowRoutes = []requestFlowRoute{
	{http.MethodPost, "/{decisionId}/confirm", "decision.confirm", map[string]string{"decisionId": "decisionId"}},
}

// clarificationFlowRoutes is relative to /v1/clarifications (a Clarification is addressed by its own id).
var clarificationFlowRoutes = []requestFlowRoute{
	{http.MethodGet, "/{clarificationId}", "clarification.get", map[string]string{"clarificationId": "clarificationId"}},
	{http.MethodPost, "/{clarificationId}/answer", "clarification.answer", map[string]string{"clarificationId": "clarificationId"}},
	{http.MethodPost, "/{clarificationId}/cancel", "clarification.cancel", map[string]string{"clarificationId": "clarificationId"}},
}

func (h requestRoutes) mountFlow(sub chi.Router, routes []requestFlowRoute) {
	for _, rt := range routes {
		sub.Method(rt.method, rt.pattern, h.bridge(rt))
	}
}

// bridge serves one route: body (POST) or query string (GET) as channel arguments, then the URL parameters.
func (h requestRoutes) bridge(rt requestFlowRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		args := map[string]any{}
		if r.Method == http.MethodGet {
			for k, v := range r.URL.Query() {
				args[k] = v[0]
			}
			if !setPageSize(w, r.URL.Query().Get("pageSize"), args) || !setRevision(w, r.URL.Query().Get("revision"), args) {
				return
			}
		} else if !decodeOptionalRequestRouteBody(w, r, &args) {
			return
		}
		if args == nil { // a JSON `null` body
			args = map[string]any{}
		}
		for param, name := range rt.params {
			args[name] = chi.URLParam(r, param)
		}
		h.call(w, r, rt.channel, args, nil)
	}
}

// decodeOptionalRequestRouteBody is decodeRequestRouteBody for routes whose body may be empty (classify, reopen ...).
func decodeOptionalRequestRouteBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestRouteBody)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body")
		return false
	}
	return true
}
