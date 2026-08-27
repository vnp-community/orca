# TASK-CR-02-05: Require confirmation to delete an already-sent annotation (BR-CR-08)

**From Solution:** SOL-CR-02
**Priority:** P1
**Service:** `annotation-service`
**File:** `backend-go/services/annotation-service/internal/usecase/delete_annotation.go`
**Depends on:** TASK-CR-02-02
**Status:** `[ ]` TODO

---

## Context

BR-CR-08 requires confirming before deleting a comment already sent to the
agent. The state (`SentToAgent`) now exists (TASK-CR-02-02); this task adds
the enforcement point so a client's delete button doesn't have to
independently track which comments were sent — it can optimistically call
delete and handle `ANNOTATION_ALREADY_SENT` by showing a confirm dialog,
then retry with `confirmed=true`.

## Changes to make

Add a `Confirmed` field to the input, and a check after the existing
`GetAnnotation` + author-OPA check, before the delete call:

```go
type DeleteAnnotationInput struct {
	ID        string
	Confirmed bool // NEW — BR-CR-08
}
```

```go
	// actor_role is intentionally "" — see update_annotation.go's Execute
	// for why (no role claim propagated into context yet).
	allowed, err := uc.opa.Decision(ctx, actorID, existing.AuthorID, "")
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "ANNOTATION_POLICY_EVAL_FAILED", "failed to evaluate authorization policy", err)
	}
	if !allowed {
		return apperrors.New(apperrors.KindPermissionDenied, "ANNOTATION_NOT_AUTHOR", "only the annotation's author (or an admin) may delete it", nil)
	}

	// BR-CR-08 — a comment already sent to the agent needs an explicit
	// confirm before it can be deleted; the client is expected to retry with
	// Confirmed=true after showing that dialog.
	if existing.SentToAgent && !in.Confirmed {
		return apperrors.New(apperrors.KindFailedPrecondition,
			"ANNOTATION_ALREADY_SENT",
			"this comment was already sent to the agent — confirm to delete anyway",
			nil)
	}

	if err := uc.repo.DeleteAnnotation(ctx, tenantID, in.ID); err != nil {
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/annotation-service
go build ./...
go test ./internal/usecase/... -run TestDeleteAnnotation -v
```

Add cases to `usecase/delete_annotation_test.go`:
- Deleting a `SentToAgent=true` annotation with `Confirmed=false` returns
  `ANNOTATION_ALREADY_SENT` and does NOT call `Repository.DeleteAnnotation`
  (assert zero calls on the fake).
- `Confirmed=true` proceeds to delete.
- Deleting a not-yet-sent (`SentToAgent=false`) annotation with
  `Confirmed=false` still succeeds — regression guard against the new check
  requiring confirmation universally.
