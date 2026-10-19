import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { KNOWLEDGE_SKILL_NAMES, probeKnowledgeSkills } from './agent-capability-skills'

let home: string
beforeEach(async () => {
  home = await mkdtemp(path.join(tmpdir(), 'orca-skills-'))
})
afterEach(async () => {
  await rm(home, { recursive: true, force: true })
})

describe('probeKnowledgeSkills', () => {
  it('reports every allowlisted skill as missing on an empty home', async () => {
    const res = await probeKnowledgeSkills(home)
    expect(res.map((s) => s.name)).toEqual([...KNOWLEDGE_SKILL_NAMES])
    expect(res.every((s) => !s.installed && s.sha256 === undefined)).toBe(true)
  })

  it('reports an installed skill with a stable content hash and ignores other skills', async () => {
    await mkdir(path.join(home, '.claude', 'skills', 'tdd-knowledge'), { recursive: true })
    await writeFile(path.join(home, '.claude', 'skills', 'tdd-knowledge', 'SKILL.md'), 'body')
    await mkdir(path.join(home, '.claude', 'skills', 'not-listed'), { recursive: true })
    await writeFile(path.join(home, '.claude', 'skills', 'not-listed', 'SKILL.md'), 'x')
    const a = await probeKnowledgeSkills(home)
    const b = await probeKnowledgeSkills(home)
    expect(a.find((s) => s.name === 'tdd-knowledge')).toMatchObject({ installed: true })
    expect(a.find((s) => s.name === 'tdd-knowledge')?.sha256).toMatch(/^[0-9a-f]{16}$/)
    expect(a).toEqual(b)
    expect(a.some((s) => s.name === 'not-listed')).toBe(false)
  })

  it('changes the hash when the skill text changes', async () => {
    const file = path.join(home, '.claude', 'skills', 'tdd-orca-task', 'SKILL.md')
    await mkdir(path.dirname(file), { recursive: true })
    await writeFile(file, 'one')
    const first = (await probeKnowledgeSkills(home)).find((s) => s.name === 'tdd-orca-task')?.sha256
    await writeFile(file, 'two')
    const second = (await probeKnowledgeSkills(home)).find(
      (s) => s.name === 'tdd-orca-task'
    )?.sha256
    expect(first).not.toBe(second)
  })
})
