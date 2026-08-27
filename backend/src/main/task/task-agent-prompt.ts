/**
 * task-agent-prompt — renders an OrcaTask into the text an agent executes.
 *
 * Shared by both execution paths (docs/guides/task-automation-orchestration-
 * integration.md §9.2): TaskAgentExecutor's single-agent spawn() path uses it
 * as the spawn command, and TaskOrchestrationBridge uses it as each seeded
 * TaskRow.spec, so a task renders identically regardless of which path runs it.
 *
 * @module main/task/task-agent-prompt
 */

import type { OrcaTask } from '../../shared/task-types'

/**
 * Build the agent prompt from a task's context.
 * Uses task.promptTemplate with ${task.*} interpolation if available,
 * otherwise auto-generates from title + description + aiContext.
 */
export function buildTaskAgentPrompt(task: OrcaTask): string {
  if (task.promptTemplate) {
    return task.promptTemplate.replace(/\$\{task\.([^}]+)\}/g, (_, key: string) => {
      const val = (task as unknown as Record<string, unknown>)[key]
      return val !== undefined ? String(val) : `\${task.${key}}`
    })
  }

  // Auto-generated prompt
  const lines: string[] = [`# Task: ${task.title}`]
  if (task.description) {lines.push(`\n## Description\n${task.description}`)}
  if (task.aiContext) {lines.push(`\n## AI Context\n${task.aiContext}`)}
  lines.push(`\n## Instructions`)
  lines.push(
    `Complete the task described above. When finished, the task status will be moved to "review".`
  )
  return lines.join('\n')
}
