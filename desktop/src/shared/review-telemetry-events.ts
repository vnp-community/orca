// Review / quality-gate telemetry schemas (FE-CV-SOL-095). Coarse enums and buckets only: never a
// repo/worktree/commit id, path, rule id, finding message, task id, person, branch or raw count.
// Spread into `eventSchemas`. Adding an enum value must ship in a schema release BEFORE the client
// that sends it (the main-process validator drops unknown values). Breaking changes get a `_v2` name.
import { z } from 'zod'

const lens = z.enum([
  'impact',
  'architecture',
  'dataflow',
  'erd',
  'storage',
  'structure',
  'contract',
  'quality',
  'requirements'
])
const verdict = z.enum(['pass', 'warn', 'fail', 'unknown'])
const countBucket = z.enum(['0', '1', '2-3', '4-10', '11+'])
const largeBucket = z.enum(['0', '1-3', '4-10', '11-30', '31+'])
const latencyBucket = z.enum(['<1m', '<5m', '<30m', '<4h', '>=4h'])
// Closed list of quality tools; anything else is reported as 'other' (free-form tool names never leave the app).
const toolEnum = z.enum([
  'eslint',
  'oxlint',
  'tsc',
  'vitest',
  'jest',
  'pytest',
  'go_test',
  'go_vet',
  'golangci_lint',
  'govulncheck',
  'osv_scanner',
  'gitleaks',
  'semgrep',
  'buf',
  'other'
])

export const reviewEventSchemas = {
  review_opened: z
    .object({
      source: z.enum(['agent_row', 'source_control', 'cmd_k', 'right_sidebar', 'notification', 'restore']),
      scope: z.enum(['merge_base', 'committed', 'custom_base']),
      lens,
      index: z.enum(['fresh', 'stale', 'missing', 'unknown']),
      after_agent_turn: z.boolean()
    })
    .strict(),
  review_lens_viewed: z.object({ lens, dwell: z.enum(['<10s', '<1m', '<5m', '>=5m']) }).strict(),
  quality_gate_viewed: z
    .object({
      verdict,
      reasons: countBucket,
      stale: z.boolean(),
      surface: z.enum(['review', 'source_control']),
      source: z.enum(['local', 'ci', 'mixed'])
    })
    .strict(),
  review_decision_made: z
    .object({
      decision: z.enum(['commit', 'create_review', 'send_to_agent', 'mark_reviewed', 'abandon']),
      latency: latencyBucket,
      used_review: z.boolean(),
      gate: z.enum(['pass', 'warn', 'fail', 'unknown', 'none']),
      open_findings: countBucket
    })
    .strict(),
  review_findings_summary: z
    .object({ shown: largeBucket, dismissed: largeBucket, waived: countBucket, resolved: largeBucket })
    .strict(),
  quality_finding_triaged: z
    .object({
      action: z.enum(['dismiss', 'waive', 'restore', 'revoke']),
      reason: z.enum(['not_applicable', 'accepted_risk', 'false_positive', 'later', 'other']),
      severity: z.enum(['error', 'warning', 'info']),
      tool: toolEnum,
      blocking: z.boolean()
    })
    .strict(),
  review_report_exported: z
    .object({
      format: z.enum(['markdown_copy', 'markdown_pr_insert', 'html_save']),
      provider: z.enum(['github', 'gitlab', 'azure-devops', 'gitea', 'none']),
      truncated: z.boolean()
    })
    .strict(),
  review_ai_summary: z
    .object({
      outcome: z.enum(['ok', 'bad_output', 'error', 'disabled']),
      level: z.enum(['metadata', 'diff']),
      cache_hit: z.boolean(),
      feedback: z.enum(['none', 'useful', 'wrong'])
    })
    .strict()
} as const

export const REVIEW_TOOL_VALUES = toolEnum.options
