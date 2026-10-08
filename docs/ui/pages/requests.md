# Requests

**Route / trigger:** `activeView === 'requests'` in the Zustand app store. Reached from the sidebar's "Requests" button (`SidebarRequestNavButton`, visible only when `requestFlowSupport === 'supported'`) or by `setActiveView('requests')` after creating/opening a request.
**Top-level component:** `RequestPage` (default export) — `frontend/src/renderer/src/components/request/RequestPage.tsx`, lazy-loaded from `App.tsx` and rendered when `activeView === 'requests'`.

## Purpose
Request -> Solution -> Plan -> Phase -> Task workflow UI: list and triage requests, confirm the AI-proposed type, review analysis (CR-REQ-020) and the plan tree (CR-REQ-021), decide pending approvals from an inbox (CR-REQ-022) and work the three backlog views (CR-REQ-023).

## Layout
```
RequestPage
├── RequestPageHeader      title · project scope select · "Create request" · close
└── Tabs: Requests | Approvals (badge) | Backlog
    └── RequestsTab        ResizablePanelGroup 40/60 (below 768px: list OR detail)
        ├── list: RequestListToolbar > RequestFilterBar, RequestList > RequestRow,
        │         RequestListStates (skeleton / empty / filtered-empty / error)
        └── RequestDetailPane
            ├── RequestDetailHeader (Cancel, Reopen, Return to backlog, Change type, Create child)
            ├── RequestBacklogBanner, RequestStageTimeline, TypeConfirmationCard
            └── Tabs: Overview | Analysis (solution/) | Plan (plan/) | History | Related
```
Dialogs: `CreateRequestDialog`, `CancelRequestDialog`, `ReturnToBacklogDialog`, `ChangeTypeConfirmDialog`, `SpawnChildRequestDialog` (all built on `RequestReasonDialog` where a reason is needed).

## Data
`requestPage` slice state (`section`, `requestId`, `listFilters`), `useRequests` (paged `request.list`), `useRequest` (`request.get` + `request.typeHistory`), `useRequestActions`, and the bus-fed `useRequestSubscription`. One global stream/poll is owned by `useRequestEvents` (mounted in `App.tsx`); screens never open their own socket.

## Keyboard
`j`/`k` or Arrow keys move the selection in the list, `Enter` opens, `Escape` closes the detail (or the page when not typing). `Cmd+Enter` on macOS / `Ctrl+Enter` elsewhere submits dialogs from the text area.

## Entry points outside the page
"Create request" buttons (`CreateRequestIssueButton`) on Jira issue rows/detail and GitHub issue detail, next to "Start workspace". They send the issue into the analysis flow without opening a worktree. Hidden for pull requests and when the runtime lacks the request flow.

## States
Unsupported runtime shows `RequestUnsupportedNotice` (no error styling); unknown support shows a skeleton; every list/detail has loading, empty and error (retry) states. Request bodies render as plain text only.

## Graph
"View graph" in `RequestDetailHeader` opens `RequestGraphSheet` (lazy `GraphPanel` > `GraphCanvas`/`GraphListView`); the Plan tab has a Tree | Graph switch (lens `plan`). Seven lenses: `flow`, `plan`, `execution` are built on the client; `architecture`, `contract`, `data`, `impact` come from `impact.graph` and their chips are disabled with a reason when there is no assessment or the runtime lacks `impact.*`. The list is a peer view (default when truncated or narrow). `/` opens node search (ignored while typing in a field); `Enter` opens node details, `Esc` leaves focus mode. Risk is always shown as text + icon + border style (`--risk-*` tokens), never color alone; "Not assessed" is a separate state and never reads as low.

## Approval inbox
`ApprovalInboxTab` (Approvals tab, `components/request/approval/`) lists the approvals the current user can decide, from `approval.listPending` (paged 50, no total; polled every 30 s while visible, plus a debounced reload on `approval.*` events). Rows are grouped by Request and ordered overdue first, then nearest due date, then newest. Filters: subject group (All / Request type / Solution / Plan / Phase / Pre-deploy / Other), overdue only, and the project scope from the page header; only single-type groups are filtered server side, the rest on the loaded pages. Request titles come from `request.get` (4 at a time, cached in `requestsById`).

- Actions: **Open** deep-links to the tab holding the gate (`requestPage.focus`: type confirmation, analysis, plan); **Quick approve** asks for confirmation first and is offered only for request type, findings, answer, task list, phase and pre-deploy approvals that carry a digest (never solution or plan); **Reject** opens `RejectReasonDialog` (reason of at least 10 characters). Decisions always send `expectedVersion` and `expectedDigest`.
- Outcomes: someone else decided or it expired -> neutral toast and the row is dropped; content changed after listing -> reload with an "Open" action; no permission -> error toast, row kept.
- Keys: `j`/`k` (or arrows) move, `Enter` opens; ignored while typing, in dialogs, during IME composition or with modifier keys. `Cmd+Enter` / `Ctrl+Enter` submits the reject dialog.
- States: skeleton (6 rows), empty, empty-with-filters (clear button), network banner over a dimmed list, forbidden message. The sidebar badge and the tab title show the count (`99+` above 99).

## Backlog
`BacklogTab` (Backlog tab, `components/request/backlog/`) has three segments, switchable with `1`, `2`, `3`; each segment owns its own hook instance and only the open one calls the server.

| Segment | Channel | Content |
|---|---|---|
| Requests | `backlog.requests` | Requests returned to the backlog: source, type, stage returned from, category, reason, who and when. **Reopen** sends the Request back to classification; **Cancel** takes an optional reason (required if the server insists). |
| Tasks | `backlog.tasks` | Tasks whose Plan/Phase is not approved, grouped by Plan with a gate badge; read only. |
| Execute | `backlog.execute` | Approved tasks not started or failed, grouped by Phase, with last error, failed runs and engine; read only (re-run from the task detail). |

Search, Request type and category filter the loaded pages client side. Counts are for loaded rows (`+` when more pages exist). `j`/`k`/`Enter` navigate rows (Enter opens the Request, or the task in a side sheet). Refresh happens on `request.returned`, `request.status_changed`, `plan.generated`, `phase.started`, `phase.completed`, `approval.decided` events and every 30 s while visible. The Task board no longer has a `backlog` column; it shows a hint with a link to this tab when the request flow is supported.

## Analysis (Solution review)
`RequestAnalysisTab` > `SolutionPanel` (`components/request/solution/`) reads `solution.list` and the Request's approvals. Types without analysis (e.g. `task`) render nothing; `hotfix` shows the result without a decision bar. A `change_request` solution shows its options as a radio group of `SolutionOptionCard`s ("Recommended" marked) with a **Compare** switch to `SolutionComparisonTable`. `SolutionDecisionBar`: pick an option, then **Approve this option** sends `solution.choose` and then `approval.approve` with the fresh `approvalDigest` from choose; **Reject** opens `RejectReasonDialog` (10+ chars, `Cmd+Enter` / `Ctrl+Enter` submits, plain Enter adds a line). Older versions collapse under "Older version" and are read only; expired approvals only offer Regenerate.

## Plan
`RequestPlanTab` (`components/request/plan/`) builds Plan > Phase > Task from `task.list` (`usePlanTree`; no `plan.*` channels) plus `approval.list`. `PlanSummaryHeader` shows progress and "N phases, M tasks"; `PlanApprovalBar` approves/rejects/regenerates the Plan (`request.generatePlan` runs `mode: 'propose'` then `mode: 'commit'` with the returned proposal); each `PhaseNode` has `PhaseApprovalBar`, where **Start phase** stays disabled until the phase approval is `approved` and then sends `request.startPhase {id, phaseTaskId}`. Approve/reject send `{id, expectedVersion, expectedDigest}`. A runtime without the task channel shows a neutral notice, never a red error. Tree | Graph switch: see Graph above.

## Tests
Vitest next to each component (`components/request/**`, `i18n/request-locale-coverage.test.ts`). Browser e2e against a mocked gateway WebSocket: `tests/e2e/request-web/*.web.e2e.ts` (support: `tests/e2e/request-web/support/mock-request-ws.ts`), run with `npx playwright test -c tests/playwright.web.config.ts --project=mcp-web tests/e2e/request-web` (set `MCP_E2E_BASE_URL` to reuse an already running Vite server).

## Clarification, decisions, risk, readiness, results
- **Clarification:** a Request in `awaiting_information` shows `ClarificationPanel` in the detail pane. Answers are submitted once for the whole round (`Mod+Enter` in a text area submits when valid); suggested defaults are only sent after "Use suggestion"; drafts live in memory only. Non-assignees see it read-only.
- **Decisions:** choosing an option other than the recommendation needs a reason (10+ chars). A high-risk Decision (server-assessed) asks to retype the option name before Approve. Plan Approve is locked until a solution Decision is effective.
- **Risk:** option cards show `RiskSummaryCard`, the option x dimension table and findings. In `enforce` mode medium risk needs the impact section opened, high/critical needs a reasoned acceptance per high finding (10+ chars); shadow mode never blocks. "Bypass the gate" (20+ chars, audited) only appears when the backend allows it.
- **Readiness and drift:** work tasks show a readiness badge (`PlanTaskRow`, `TaskDetail`) and a per-phase summary; a task that is not ready locks Run. A pending phase approval at `drift_review` shows `PlanDriftBanner`; the review sheet offers accept (approve) or return (reject), never "cancel phase".
- **Results:** `TaskDetail` has a Result tab for Request-owned tasks (hidden when the runtime lacks `execution.get`): summary, files in/out of scope, agent-reported vs Orca re-run checks, secret scan state, failure routing. All text is plain text.

