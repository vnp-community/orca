// src/relay/agent-capability-skills.ts
// Reports which skills Orca relies on are installed for claude on this host (the knowledge hub, and the diagram-design skill the
// Outcome Review draws with), by name and content hash only. Names come from a fixed allowlist; nothing outside it is ever read.

import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import * as os from 'node:os'
import path from 'node:path'

export const KNOWLEDGE_SKILL_NAMES = [
  'tdd-knowledge',
  'tdd-openspec-author',
  'tdd-knowledge-writeback',
  'tdd-orca-task',
  'diagram-design'
] as const

export type SkillEntry = { name: string; installed: boolean; sha256?: string }

export async function probeKnowledgeSkills(homeDir: string = os.homedir()): Promise<SkillEntry[]> {
  return Promise.all(
    KNOWLEDGE_SKILL_NAMES.map(async (name): Promise<SkillEntry> => {
      try {
        const body = await readFile(path.join(homeDir, '.claude', 'skills', name, 'SKILL.md'))
        return {
          name,
          installed: true,
          sha256: createHash('sha256').update(body).digest('hex').slice(0, 16)
        }
      } catch {
        return { name, installed: false }
      }
    })
  )
}
