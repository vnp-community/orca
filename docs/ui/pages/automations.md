# Automations

**Route / trigger:** `activeView === 'automations'`. Opened via the sidebar "Automations" button (`frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`, `CalendarClock` icon, calls `openAutomationsPage()`), shown by default (hideable per-user via `settings.showAutomationsButton`, with a right-click "Hide" menu). Also opened contextually from a worktree card's menu (`frontend/src/renderer/src/components/sidebar/WorktreeCard.tsx`) to jump straight into automations scoped to that project.
**Top-level component:** `AutomationsPage` (`frontend/src/renderer/src/components/automations/AutomationsPage.tsx`, ~3000 lines)

## Purpose
Manage scheduled/recurring AI agent runs ("automations"): create a cron-like schedule that dispatches a prompt to an agent in a given project/workspace, inspect past run results, and (for supported providers) manage externally-hosted automations (Hermes cron jobs) alongside Orca-native ones.

## Layout
```
┌ header: Close · "Automations" title · [+ Add] ─────────── [Refresh] ┐
├───────────────────────────┬───────────────────────────────────────┤
│ left list (280-360px)     │ right panel                            │
│  - Orca automations       │  Tabs: [Overview] [Runs (n)]           │
│    (grouped by project)   │   Overview → AutomationDetail          │
│  - External automations   │   Runs → AutomationRunHistory list,    │
│    (e.g. Hermes cron)     │     or a single run drilldown          │
│                            │     (AutomationRunPageFrame)           │
└───────────────────────────┴───────────────────────────────────────┘
```
- Header: close button, `CalendarClock` icon + "Automations" title, "Add automation" button (opens `AutomationEditorDialog`), and a refresh button.
- Left column (`grid-cols-[minmax(280px,360px)_1fr]`): a list of native `automations` (from `automations.map`, grouped via `automation-project-groups.ts`) followed by `externalAutomationEntries` (external manager sources/jobs, e.g. Hermes). Each row shows name, schedule summary, and status badges.
- Right panel: a `Tabs` component with **Overview** (renders `AutomationDetail.tsx` — schedule, project, workspace, agent, prechecks, run-now/edit/toggle/delete actions) and **Runs** (renders `AutomationRunHistory.tsx`, a list of past `AutomationRun`s; clicking one opens `AutomationRunPageFrame.tsx` showing that run's captured output via `CommentMarkdown`).
- Modals/dialogs layered on top: `AutomationEditorDialog.tsx` (create/edit form: name, prompt, agent, project, workspace mode, schedule, precheck, setup decision — composed from `AutomationSchedulePicker`, `AutomationCustomCronPanel`, `AutomationPrecheckFields`, `AutomationProjectCombobox`, `WorkspaceCombobox`, `AutomationSetupDecisionField`, `AutomationSessionField`, `AutomationMissedRunGraceField`), a delete-confirmation `Dialog`, and external-source connect/edit/delete dialogs (`ExternalAutomationManagers.tsx`, `CreateFromPicker.tsx`).

## Data shown
- **Automations list**: `automations: Automation[]` (local `useState`, loaded via `window.api.automations.list()`). Shape (`frontend/src/shared/automations-types.ts`): `id, name, prompt, precheck (command+timeoutSeconds) | null, agentId (TuiAgent), runContext/sourceContext (host+project identity), projectId (legacy), executionTargetType ('local'|'ssh'), executionTargetId, schedulerOwner, workspaceMode ('existing'|'new_per_run'), workspaceId, baseBranch, setupDecision, reuseSession, timezone, rrule, dtstart, enabled, nextRunAt, lastRunAt, missedRunPolicy, missedRunGraceMinutes, createdAt, updatedAt`.
- **Runs**: `runs: AutomationRun[]` / `selectedAutomationRuns` (via `window.api.automations.listRuns({automationId})`). Each run has `status` (`pending|dispatching|dispatched|completed|skipped_precheck|skipped_missed|skipped_unavailable|skipped_needs_interactive_auth|dispatch_failed`), `trigger ('scheduled'|'manual')`, `precheckResult`, `outputSnapshot` (captured plain-text output), `usage` (token/cost estimate per `AutomationRunUsage`), and workspace linkage.
- **External automations**: `externalManagers: ExternalAutomationManager[]` via `window.api.automations.listExternalManagers()`, each with `.jobs: ExternalAutomationJob[]`; run pages for these come from `window.api.automations.listExternalRuns()`.
- Cross-referenced store state: `repos`, `projectHostSetups`, `worktreesByRepo`, `unifiedTabsByWorktree`, `terminalLayoutsByTabId`, `ptyIdsByTabId`, `agentStatusByPaneKey`, `retainedAgentsByPaneKey`, `sshConnectionStates`, `runtimeEnvironments`/`runtimeStatusByEnvironmentId` (used to resolve run-now availability and open-workspace targets), `preflightStatus` (hook/setup readiness), `selectedAutomationId`/`pendingAutomationRunNavigation` (deep-link navigation state in the UI store).

## Key interactions
- **Add automation**: opens `AutomationEditorDialog` with a blank draft; pick project, agent, prompt, workspace mode (existing worktree vs. new-per-run), base branch, schedule preset (`hourly|daily|weekdays|weekly|custom` via `AutomationSchedulePicker`/`AutomationCustomCronPanel`), optional precheck command, setup decision, missed-run grace window. Saves via `window.api.automations.create()`.
- **Edit**: same dialog pre-filled, saves via `window.api.automations.update({id, updates})`.
- **Delete**: confirmation dialog (with a "don't ask again" checkbox), calls `window.api.automations.delete({id})`; explains that run history is deleted but created workspaces are not.
- **Toggle enable/disable**, **Run now** (`window.api.automations.runNow({id})`), all from `AutomationDetail`.
- **Runs tab**: browse history (`AutomationRunHistory`), open a specific run's captured output, **Rerun** a completed run (`rerunAutomationRun`, guarded by `canRerunAutomationRun`), or **Open workspace** to jump to the terminal/workspace the run executed in (`canOpenAutomationRunOpenTarget` checks the target tab/pty is still live).
- **External automations**: connect a source (Hermes), create/update/delete external jobs, and run manual actions (`window.api.automations.runExternalAction`).

## Notable implementation details / known issues
- Automations can be scoped to `local` or `ssh` execution targets and to `runtime` (DevServer/remote) hosts; the page probes runtime host preflight status per unique host (`runtimePreflightStatusByHostId`) so it can show "host unavailable" without duplicate probes per automation.
- `projectId` on `Automation` is explicitly legacy/deprecated in favor of `runContext.projectId`/`runContext.repoId` — new code should prefer `runContext`.
- Deep-link navigation (`pendingAutomationRunNavigation`, set elsewhere e.g. from a notification) is resolved in a `useEffect` that selects the right automation/run tab once data has loaded, and shows a toast if the target automation/run no longer exists.
- This is by far the largest of the five pages (~3000 lines) — most of the file is state/effects for editor-draft management, setup-decision defaulting, and run/host preflight orchestration rather than rendering.
