# TASK-CR-02-07: Wire new annotation fields and `annotation.markSent` through wscompat and REST

**From Solution:** SOL-CR-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go`
**Depends on:** TASK-CR-02-01, TASK-CR-02-04, TASK-CR-02-05, TASK-CR-02-06
**Status:** `[x]` DONE — annotationAnchorArg/create/list/delete extended, annotation.markSent + POST /v1/annotations/mark-sent wired; go build/vet/test pass (channels_test.go, annotation_routes_test.go).

---

## Context

`registerAnnotationChannels` (`channels.go:138-220`) and
`mountAnnotationRoutes`/its handlers (`annotation_routes.go`) only know the
pre-SOL-CR-02 `Anchor`/request shapes. This task threads the new fields
through both transports and adds the `annotation.markSent`
channel / `POST /v1/annotations/mark-sent` route.

## Changes to make

### 1. `wscompat/channels.go`

Extend `annotationAnchorArg` and `annotation.create`:

```go
type annotationAnchorArg struct {
	RepoID     string `json:"repoId"`
	WorktreeID string `json:"worktreeId"` // NEW
	FilePath   string `json:"filePath"`
	Line       int32  `json:"line"`
	EndLine    int32  `json:"endLine"` // NEW
	Side       int32  `json:"side"`    // NEW
	Ref        string `json:"ref"`
}
```

```go
	r.Register("annotation.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Anchor       annotationAnchorArg `json:"anchor"`
			Content      string              `json:"content"`
			RequestID    string              `json:"requestId"`
			OriginalCode string              `json:"originalCode"` // NEW
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateAnnotation(ctx, &annotationv1.CreateAnnotationRequest{
			Anchor: &annotationv1.Anchor{
				RepoId:     in.Anchor.RepoID,
				WorktreeId: in.Anchor.WorktreeID,
				FilePath:   in.Anchor.FilePath,
				Line:       in.Anchor.Line,
				EndLine:    in.Anchor.EndLine,
				Side:       annotationv1.Side(in.Anchor.Side),
				Ref:        in.Anchor.Ref,
			},
			Content:      in.Content,
			RequestId:    in.RequestID,
			OriginalCode: in.OriginalCode,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAnnotation(), nil
	})
```

Extend `annotation.list`:

```go
	r.Register("annotation.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			RepoID      string `json:"repoId"`
			FilePath    string `json:"filePath"`
			PageToken   string `json:"pageToken"`
			PageSize    int32  `json:"pageSize"`
			WorktreeID  string `json:"worktreeId"`  // NEW
			SentToAgent *bool  `json:"sentToAgent"` // NEW
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &annotationv1.ListAnnotationsRequest{
			RepoId:     in.RepoID,
			FilePath:   in.FilePath,
			PageToken:  in.PageToken,
			PageSize:   in.PageSize,
			WorktreeId: in.WorktreeID,
		}
		if in.SentToAgent != nil {
			req.SentToAgent = proto.Bool(*in.SentToAgent)
		}
		resp, err := client.ListAnnotations(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

(Add `"google.golang.org/protobuf/proto"` to this file's imports if not
already present — needed for `proto.Bool`.)

Extend `annotation.delete`:

```go
	r.Register("annotation.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID        string `json:"id"`
			Confirmed bool   `json:"confirmed"` // NEW
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteAnnotation(ctx, &annotationv1.DeleteAnnotationRequest{Id: in.ID, Confirmed: in.Confirmed}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
```

Add a new channel, registered at the end of `registerAnnotationChannels`:

```go
	r.Register("annotation.markSent", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type markSentArgs struct {
			IDs []string `json:"ids"`
		}
		in, err := decodeArg[markSentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.MarkAnnotationsSent(ctx, &annotationv1.MarkAnnotationsSentRequest{Ids: in.IDs})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

### 2. `httpgateway/annotation_routes.go`

Add `worktree_id`/`end_line`/`side` to `annotationAnchorBody` and
`original_code` to `createAnnotationRequestBody`, threaded into
`CreateAnnotationRequest` the same way `handleCreateAnnotation` already
does. Add `confirmed` (query param `?confirmed=true`) to
`handleDeleteAnnotation`. Add a new route + handler:

```go
	sub.Post("/mark-sent", handleMarkAnnotationsSent(client))
```

```go
type markAnnotationsSentRequestBody struct {
	IDs []string `json:"ids"`
}

func handleMarkAnnotationsSent(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body markAnnotationsSentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.MarkAnnotationsSent(ctx, &annotationv1.MarkAnnotationsSentRequest{Ids: body.IDs})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/wscompat/... -run TestAnnotation -v
go test ./internal/adapter/httpgateway/... -run TestAnnotation -v
```

Add cases: `annotation.delete` forwards `confirmed`; `annotation.markSent`
wired and forwards `ids`; `POST /v1/annotations/mark-sent` forwards `ids`
and returns the updated annotations.
