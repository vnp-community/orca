/**
 * TaskAIPlanner — AI-powered task decomposition and prompt generation (TDD-18)
 *
 * Uses the project's AI provider (via relay ai.complete) to:
 * 1. Decompose a task into 3-7 concrete subtasks
 * 2. Apply decomposition (create as child tasks)
 * 3. Generate prompt template for agent execution
 *
 * Relay call: relay.call('ai.complete', { prompt, format: 'json' })
 *
 * @module main/task/TaskAIPlanner
 */

import type { TaskService } from './TaskService'
import type { AIProviderService } from '../ai-providers/AIProviderService'
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { OrcaTask } from '../../shared/task-types'

// ── Decomposed subtask shape from AI response ─────────────────────────────────

interface SubtaskProposal {
  title: string
  type?: 'subtask' | 'task'
  estimatedHours?: number
  description?: string
}

// ── TaskAIPlanner ─────────────────────────────────────────────────────────────

export class TaskAIPlanner {
  constructor(
    private readonly taskService: TaskService,
    private readonly providerService: AIProviderService,
    private readonly router: ProjectServerRouter
  ) {}

  /**
   * Decompose a task into subtasks using AI.
   * Calls relay ai.complete with a structured prompt.
   * Returns list of proposed subtasks (NOT yet persisted).
   *
   * @param taskId - Parent task to decompose
   * @param projectId - Project context for relay routing
   * @param userId - User requesting decomposition
   */
  async decompose(taskId: string, projectId: string, userId: string): Promise<OrcaTask[]> {
    const task = await this.taskService.get(taskId)
    if (!task) throw new Error(`TASK_NOT_FOUND: ${taskId}`)

    const prompt = this.buildDecomposePrompt(task)

    // Route to relay and call ai.complete
    const relay = await this.router.getRelayForProject(projectId, userId)
    const response = (await relay.call('ai.complete', {
      prompt,
      format: 'json',
      taskId,
    })) as { content?: string; text?: string } | string

    // Parse AI response
    const proposals = this.parseAIResponse(response)

    // Convert proposals to OrcaTask-shaped objects (not persisted)
    return proposals.map((p) => ({
      id: `proposed:${Date.now()}:${Math.random().toString(36).slice(2)}`,
      parentId: taskId,
      projectId: task.projectId,
      title: p.title,
      description: p.description,
      type: p.type ?? 'subtask',
      status: 'backlog' as const,
      priority: task.priority,
      labels: [],
      visibility: task.visibility,
      progressPercent: 0,
      estimatedHours: p.estimatedHours,
      createdAt: new Date(),
      updatedAt: new Date(),
    }))
  }

  /**
   * Apply AI-generated subtasks: create them as children of the parent task.
   * @param taskId - Parent task ID
   * @param subtasks - Partial OrcaTask array (typically from decompose())
   * @returns Persisted OrcaTask[] children
   */
  async applyDecomposition(
    taskId: string,
    subtasks: Array<Partial<OrcaTask>>
  ): Promise<OrcaTask[]> {
    const parent = await this.taskService.get(taskId)
    if (!parent) throw new Error(`TASK_NOT_FOUND: ${taskId}`)

    const created: OrcaTask[] = []
    for (const s of subtasks) {
      const child = await this.taskService.create({
        parentId: taskId,
        projectId: parent.projectId,
        title: s.title ?? 'Untitled subtask',
        description: s.description,
        type: s.type ?? 'subtask',
        priority: s.priority ?? parent.priority,
        labels: s.labels,
        visibility: s.visibility ?? parent.visibility,
        estimatedHours: s.estimatedHours,
        aiContext: s.aiContext,
      })
      created.push(child)
    }
    return created
  }

  /**
   * Generate an agent prompt template for a task based on its context.
   * Returns a string template with ${task.*} placeholders resolved.
   */
  async generatePromptTemplate(taskId: string, userId: string): Promise<string> {
    const task = await this.taskService.get(taskId)
    if (!task) throw new Error(`TASK_NOT_FOUND: ${taskId}`)

    // If task already has a prompt template, resolve it
    if (task.promptTemplate) {
      return this.interpolateTemplate(task.promptTemplate, task)
    }

    // Auto-generate minimal prompt from task fields
    const lines: string[] = [
      `# Task: ${task.title}`,
    ]
    if (task.description) lines.push(`\n## Description\n${task.description}`)
    if (task.aiContext) lines.push(`\n## Context\n${task.aiContext}`)
    lines.push(`\n## Type: ${task.type} | Priority: ${task.priority} | Status: ${task.status}`)
    lines.push(`\nPlease complete this task. When done, update the status to 'review'.`)

    return lines.join('\n')
  }

  // ── Private helpers ────────────────────────────────────────────────────────

  private buildDecomposePrompt(task: OrcaTask): string {
    return [
      `You are a software project manager. Decompose the following task into 3-7 concrete subtasks.`,
      `Return a JSON array only, no explanation: [{ "title": string, "type": "subtask"|"task", "estimatedHours": number, "description": string }]`,
      ``,
      `Task: ${task.title}`,
      task.description ? `Description: ${task.description}` : '',
      task.aiContext ? `Context: ${task.aiContext}` : '',
    ].filter(Boolean).join('\n')
  }

  private parseAIResponse(response: unknown): SubtaskProposal[] {
    try {
      let text = ''
      if (typeof response === 'string') {
        text = response
      } else if (response && typeof response === 'object') {
        const r = response as Record<string, unknown>
        text = (r['content'] ?? r['text'] ?? '') as string
      }

      // Extract JSON array from response
      const match = text.match(/\[[\s\S]*\]/)
      if (!match) return []
      return JSON.parse(match[0]) as SubtaskProposal[]
    } catch {
      console.warn('[TaskAIPlanner] Failed to parse AI response')
      return []
    }
  }

  private interpolateTemplate(template: string, task: OrcaTask): string {
    return template.replace(/\$\{task\.([^}]+)\}/g, (_, key: string) => {
      const val = (task as unknown as Record<string, unknown>)[key]
      return val !== undefined ? String(val) : `\${task.${key}}`
    })
  }
}
