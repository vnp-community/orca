# TASK-WF-01-07: Wire authoring fields + Clone route into REST gateway

**From Solution:** SOL-WF-01
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go`
**Depends on:** TASK-WF-01-03, TASK-WF-01-05
**Status:** `[ ]` TODO

---

## Context

`createTemplateRequestBody` and `handleCreateTemplate` don't yet expose
the new authoring fields over REST, and there is no route for
`CloneTemplate`.

## Changes to make

In `workflow_routes.go`, extend `createTemplateRequestBody` (line ~44-49):

```go
type createTemplateRequestBody struct {
    // ... existing fields unchanged ...
    Description     string   `json:"description"`
    Tags             []string `json:"tags"`
    OverridesJSON    string   `json:"overridesJson"`
    InjectStepsJSON  string   `json:"injectStepsJson"`
    RemoveStepsJSON  string   `json:"removeStepsJson"`
}
```

Thread these into `handleCreateTemplate`'s `CreateTemplateRequest`
construction (line ~62-68). Apply the identical field set to
`updateTemplateRequestBody`/`handleUpdateTemplate`.

Add the Clone route inside `mountWorkflowRoutes` (line ~26-37), following
the existing `chi.URLParam(r, "id")` pattern used by
`handleGetExecution`/`handlePauseExecution`:

```go
sub.Post("/templates/{id}/clone", handleCloneTemplate(client))
```

```go
func handleCloneTemplate(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        var body struct {
            Name, Description string
            Tags              []string
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            httpError(w, http.StatusBadRequest, err)
            return
        }
        resp, err := client.CloneTemplate(r.Context(), &workflowv1.CloneTemplateRequest{
            SourceTemplateId: id, Name: body.Name, Description: body.Description, Tags: body.Tags,
        })
        if err != nil {
            httpError(w, grpcStatusToHTTP(err), err)
            return
        }
        writeJSON(w, http.StatusCreated, resp.GetTemplate())
    }
}
```

(Match `httpError`/`grpcStatusToHTTP`/`writeJSON` to whatever helpers this
file already uses — confirm exact names before implementing.)

Note: `wscompat`'s `workflow.*` namespace has zero channel registrations
today (tracked separately by BUG-030) — do not add `CloneTemplate`
wscompat wiring in this task; leave it for whoever picks up BUG-030.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestWorkflowRoutes
```

Expected: `POST /v1/workflows/templates/{id}/clone` happy path returns
201 with the new template, and a 404 on an unknown `source_template_id`;
`POST`/`PUT` create/update template requests round-trip
description/tags/overrides/inject/remove fields end-to-end.
