/**
 * TemplateResolver — Workflow template inheritance chain resolver (TDD-17)
 *
 * Templates can inherit from parent templates (parentTemplateId) to compose
 * reusable workflow definitions. Leaf overrides root — the chain is merged
 * from root to leaf so child steps shadow parent steps with the same id.
 *
 * MAX_INHERIT_DEPTH = 5 prevents infinite loops.
 *
 * DB table: orca_workflow_templates
 *   id, name, definition (JSON), owner_id, scope, parent_template_id, created_at
 *
 * @module main/workflow/TemplateResolver
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { WorkflowDefinition, WorkflowStep } from './WorkflowTypes'

const MAX_INHERIT_DEPTH = 5

interface TemplateRow {
  id: string
  name: string
  definitionJson: string   // JSON — column: definition_json
  ownerId: string
  scope: string
  parentTemplateId: string | null
  createdAt: number
}

export interface CreateTemplateParams {
  name: string
  definition: WorkflowDefinition
  ownerId: string
  scope?: string
  parentTemplateId?: string
}

export interface TemplateRecord {
  id: string
  name: string
  definition: WorkflowDefinition
  ownerId: string
  scope: string
  parentTemplateId?: string
  createdAt: Date
}

function rowToRecord(r: TemplateRow): TemplateRecord {
  return {
    id: r.id,
    name: r.name,
    definition: JSON.parse(r.definitionJson) as WorkflowDefinition,
    ownerId: r.ownerId,
    scope: r.scope,
    parentTemplateId: r.parentTemplateId ?? undefined,
    createdAt: new Date(r.createdAt),
  }
}

export class TemplateResolver {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * Resolve a template by ID, following the inheritance chain.
   * Merges from root → leaf (leaf overrides root).
   *
   * @throws Error if templateId is not found
   * @throws Error if inheritance depth exceeds MAX_INHERIT_DEPTH
   */
  async resolve(templateId: string): Promise<WorkflowDefinition> {
    // Load the chain from leaf → root
    const chain: WorkflowDefinition[] = []
    let currentId: string | null = templateId
    let depth = 0

    while (currentId !== null) {
      if (depth > MAX_INHERIT_DEPTH) {
        throw new Error(
          `TEMPLATE_INHERIT_DEPTH_EXCEEDED: inheritance chain depth exceeds ${MAX_INHERIT_DEPTH} for template "${templateId}"`
        )
      }

      const rows = await this.pool.withConnection((db) =>
        db.query<TemplateRow>(
          `SELECT id, name, definition_json as definitionJson, owner_id as ownerId, scope,
                  parent_template_id as parentTemplateId, created_at as createdAt
           FROM orca_workflow_templates WHERE id = ?`,
          [currentId]
        )
      )

      if (!rows[0]) {
        if (depth === 0) {
          throw new Error(`TEMPLATE_NOT_FOUND: "${templateId}"`)
        }
        // Broken chain — parent no longer exists, stop here
        break
      }

      const template = rows[0]
      chain.unshift(JSON.parse(template.definitionJson) as WorkflowDefinition) // prepend so chain is root→leaf
      currentId = template.parentTemplateId
      depth++
    }

    if (chain.length === 0) {
      throw new Error(`TEMPLATE_NOT_FOUND: "${templateId}"`)
    }

    // Merge chain: root → leaf (later entries override earlier)
    return this.mergeDefinitions(chain)
  }

  /**
   * Create a new workflow template.
   * @returns The new template ID
   */
  async create(params: CreateTemplateParams): Promise<string> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_workflow_templates
           (id, name, definition_json, owner_id, scope, parent_template_id, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id,
          params.name,
          JSON.stringify(params.definition),
          params.ownerId,
          params.scope ?? 'user',
          params.parentTemplateId ?? null,
          now,
          now,
        ]
      )
    )
    return id
  }

  /**
   * List templates by scope, optionally filtered by ownerId.
   */
  async list(scope: string, ownerId?: string): Promise<TemplateRecord[]> {
    let sql = `SELECT id, name, definition_json as definitionJson, owner_id as ownerId, scope,
                      parent_template_id as parentTemplateId, created_at as createdAt
               FROM orca_workflow_templates WHERE scope = ?`
    const params: unknown[] = [scope]

    if (ownerId) {
      sql += ' AND owner_id = ?'
      params.push(ownerId)
    }
    sql += ' ORDER BY created_at DESC'

    const rows = await this.pool.withConnection((db) =>
      db.query<TemplateRow>(sql, params)
    )
    return rows.map(rowToRecord)
  }

  // ── Private ──────────────────────────────────────────────────────────────

  /**
   * Merge multiple WorkflowDefinitions from root to leaf.
   * Leaf steps override root steps with the same ID.
   * Inputs are shallow-merged (leaf wins per key).
   */
  private mergeDefinitions(chain: WorkflowDefinition[]): WorkflowDefinition {
    const stepMap = new Map<string, WorkflowStep>()
    let mergedInputs: Record<string, unknown> = {}

    for (const definition of chain) {
      // Merge inputs (leaf wins per key)
      if (definition.inputs) {
        mergedInputs = { ...mergedInputs, ...definition.inputs }
      }
      // Merge steps (leaf overrides steps with same id)
      for (const step of definition.steps) {
        stepMap.set(step.id, step)
      }
    }

    return {
      steps: [...stepMap.values()],
      inputs: Object.keys(mergedInputs).length > 0 ? mergedInputs : undefined,
    }
  }
}
