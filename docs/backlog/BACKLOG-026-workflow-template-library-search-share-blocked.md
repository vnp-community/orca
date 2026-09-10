# BACKLOG-026: Workflow Template Library's search/sort/share — blocked on unbuilt backend RPCs

**Origin:** `specs/frontend/crs/v4/workflow/tasks/FE-TASK-002-workflow-template-library.md` (FE-SOL-001, CR-WF-006)
**Priority:** Low — Browse and Use (the core flow) work today against the real `workflow.template.list` RPC; only the enhancement features are missing
**Blocked on:** `BE-SOL-005` (`template-sharing-library-and-list-executions`, backend-go, **📋 Proposed**) — full-text search, trending sort, and sharing/cloning RPCs don't exist
**Owner:** whoever owns backend-go's workflow-service roadmap

---

## What this is

`WorkflowLibrary` (built 2026-09-09) lists and lets a user apply workflow
templates today, backed by the real `workflow.template.list` RPC — no
gaps in the base Browse/Use flow.

## Why part of it is limited

`workflow.template.list` has no search or sort parameters, and no
sharing/cloning RPC exists — both are scoped into `BE-SOL-005`, which is
still `📋 Proposed`. Until it ships, the Library can only show templates
in list order with client-side substring filtering (not real full-text
search or "trending" ranking), and Share/Clone aren't available.

## What unblocks this

`BE-SOL-005` ships search/sort parameters on `workflow.template.list` (or
a dedicated `SearchTemplates` RPC) plus sharing RPCs. Once it exists,
extending `WorkflowLibrary`/`useWorkflowLibrary` to use them is additive —
no redesign needed, the current UI already has a place for these controls.
