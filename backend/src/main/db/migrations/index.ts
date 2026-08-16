/**
 * Migration Registry
 *
 * ALL_MIGRATIONS is the canonical ordered list of all migrations.
 * Import this in server-bootstrap to auto-migrate.
 *
 * @module db/migrations/index
 */

import { migration0001InitialSchema } from './0001_initial_schema'
import { migration0002AddAutomations } from './0002_add_automations'
import { migration0003AddWorkspaceSessions } from './0003_add_workspace_sessions'
import { migration0004OrcaAppTables } from './0004_orca_app_tables'
import { migration0005AddAuthSchema } from './0005_add_auth_schema'
// ─── v5.0 migrations ────────────────────────────────────────────────────────
import { migration0006CompanyDept } from './0006_company_dept'
import { migration0007Projects } from './0007_projects'
import { migration0008AiProviders } from './0008_ai_providers'
import { migration0009Workflows } from './0009_workflows'
import { migration0010Tasks } from './0010_tasks'
import { migration0011TerminalSessions } from './0011_terminal_sessions'
import { migration0012PortForwardsPush } from './0012_port_forwards_push'
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'
import { migration0014WorkflowPauseState } from './0014_workflow_pause_state'
import { migration0015AiProviderRotation } from './0015_ai_provider_rotation'
import { migration0016TeamProjectSharingTaskExec } from './0016_team_project_sharing_task_exec'
import { migration0017TeamProfileJson } from './0017_team_profile_json'
import { migration0018Annotations } from './0018_annotations'
// ─── ADR-021 Phase 0 — unified Postgres / microservice-schema scaffolding ────
import { migration0019ServiceSchemasTenantRetrofit } from './0019_service_schemas_tenant_retrofit'
import { migration0020OrchestrationSchema } from './0020_orchestration_schema'
import { migration0021AutomationSchema } from './0021_automation_schema'
import { migration0022NotificationUsageSchema } from './0022_notification_usage_schema'
import { migration0023UsageStateBlob } from './0023_usage_state_blob'
import { migration0024OrcaDataStateBlob } from './0024_orca_data_state_blob'
import { migration0025OpenCodeUsageStateBlob } from './0025_opencode_usage_state_blob'
import type { Migration } from './types'

/** All migrations in version order. */
export const ALL_MIGRATIONS: readonly Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema,
  // v5.0 — Profile Hierarchy, Projects, AI Providers, Workflows, Task Graph
  migration0006CompanyDept,
  migration0007Projects,
  migration0008AiProviders,
  migration0009Workflows,
  migration0010Tasks,
  migration0011TerminalSessions,
  // v5.1 — Port Forward persistence + Push Subscriptions (BUG-BE-SSH-002, TASK-MB-001)
  migration0012PortForwardsPush,
  // v5.1 — Workflow Trace Correlation (CR-TRACE-017 §3.1 — parentTraceId resume-after-restart)
  migration0013WorkflowTraceCorrelation,
  // v5.1 — Workflow Pause State (BUG-BE-HLD-009 — user-triggered pause/resume)
  migration0014WorkflowPauseState,
  // v5.1 — AI Provider Key Rotation (BUG-BE-HLD-014 — shadow-id staging + grace period)
  migration0015AiProviderRotation,
  // v5.2 — Team metadata + priority, OrcaProject↔Project sharing, Task execution pipeline linkage
  migration0016TeamProjectSharingTaskExec,
  // v5.2 — Team profile storage (agent/editor/shell/mcp; ProfileResolver cascade-merge source)
  migration0017TeamProfileJson,
  // BL-CR-02 — Inline code-review annotations (orca_annotations backing annotation.list/annotation.create)
  migration0018Annotations,
  // ADR-021 Phase 0 — service schemas + tenant_id retrofit + orchestration/automation/
  // notification/usage schema scaffolding (specs/backend/models/08-postgres-microservices-target-architecture.md)
  migration0019ServiceSchemasTenantRetrofit,
  migration0020OrchestrationSchema,
  migration0021AutomationSchema,
  migration0022NotificationUsageSchema,
  // ADR-021 — whole-state blob table backing the actual ClaudeUsageStore/
  // CodexUsageStore Postgres swap (see 0023's module doc comment for why this
  // exists alongside 0022's granular tables instead of using them directly)
  migration0023UsageStateBlob,
  // ADR-021 — whole-state blob table backing the actual Store (persistence.ts)
  // Postgres swap (see 0024's module doc comment)
  migration0024OrcaDataStateBlob,
  // ADR-021 — whole-state blob table backing OpenCodeUsageStore's Postgres
  // swap, added after 0023 (claude/codex) since migrations are append-only
  // (see 0025's module doc comment)
  migration0025OpenCodeUsageStateBlob,
]

export { MigrationRunner } from './runner'
export type { Migration, AppliedMigration, MigrationResult } from './types'

