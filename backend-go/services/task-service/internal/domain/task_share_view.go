package domain

// TaskShareView is the DEDICATED, narrow projection GetTaskByShareToken
// returns — deliberately NOT domain.Task with fields blanked out, so a
// future field added to Task can never leak through this public,
// unauthenticated path by accident (TASK-TG-003-05). Per BE-SOL-003: only
// id/title/status/description — NEVER ai_context, ai_plan_json, comments,
// grants, owner_id, assignee_id, reporter_id, or any other Task field.
// SECURITY REVIEW REQUIRED before merge — see that task's own header.
type TaskShareView struct {
	ID          string
	Title       string
	Status      string
	Description string
}
