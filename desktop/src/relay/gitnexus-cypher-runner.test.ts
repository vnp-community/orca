import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderCypherTemplate, runCypherTemplate } from './gitnexus-cypher-runner'
import { CYPHER_TEMPLATES } from './gitnexus-cypher-templates'
import { CodeIntelError } from './codeintel-errors'
import * as ToolRunner from './codeintel-tool-runner'

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

describe('gitnexus-cypher-runner', () => {
  describe('renderCypherTemplate', () => {
    it('renders all templates and guards them', () => {
      // Create some dummy slot values that cover all types
      const dummySlots: Record<string, any> = {
        topN: 10,
        maxEdges: 50,
        offset: 0,
        limit: 10,
        id: 'dummy_id',
        ids: ['id1', 'id2'],
        path: 'dummy/path',
        paths: ['dummy/path1', 'dummy/path2'],
        kinds: ['CALLS', 'IMPORTS'],
        frontier: ['f1', 'f2']
      }

      for (const tpl of Object.values(CYPHER_TEMPLATES)) {
        const slotsNeeded: Record<string, any> = {}
        for (const key of Object.keys(tpl.slots)) {
          slotsNeeded[key] = dummySlots[key]
        }
        const rendered = renderCypherTemplate(tpl, slotsNeeded)
        expect(rendered).not.toMatch(/\{\{.*?\}\}/)
        expect(rendered.startsWith('MATCH ')).toBe(true)
      }
    })

    it('throws if missing slot value', () => {
      const tpl = CYPHER_TEMPLATES['OV_CLUSTERS']
      expect(() => renderCypherTemplate(tpl, {})).toThrow(/Missing slot value/)
    })

    it('throws if template produces forbidden query', () => {
      const badTpl: any = {
        id: 'BAD',
        text: 'MATCH (p:Process) DELETE p',
        slots: {},
        columns: []
      }
      expect(() => renderCypherTemplate(badTpl, {})).toThrow(/Forbidden Cypher keyword/)
    })
  })

  describe('runCypherTemplate', () => {
    const mockBinding: any = { mainCheckoutRoot: '/repo' }
    const mockCtx: any = {
      config: { toolEnv: {} },
      signal: new AbortController().signal,
      deadline: Date.now() + 10000,
      perf: { recordCli: vi.fn() }
    }

    beforeEach(() => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockReset()
    })

    it('runs cypher tool and parses output', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockImplementation(async (...args: any[]) => {
        const perfCtx = args[5]
        if (perfCtx?.perf) {
          perfCtx.perf.push({ tool: 'gitnexus', command: 'cypher', ms: 100, stdoutBytes: 2 })
        }
        return {
          stdout: '[]',
          stderr: '',
          exitCode: 0,
          durationMs: 100,
          stdoutBytes: 2
        }
      })

      const res = await runCypherTemplate(mockBinding, 'PR_COUNT', {}, mockCtx)
      expect(ToolRunner.runCodeIntelTool).toHaveBeenCalledWith(
        expect.arrayContaining(['cypher', expect.stringContaining('MATCH')]),
        '/repo',
        mockCtx.config,
        expect.anything(),
        expect.objectContaining({ tool: 'gitnexus' }),
        expect.anything()
      )
      expect(res.rows).toEqual([])
      expect(mockCtx.perf.recordCli).toHaveBeenCalledWith({ tool: 'gitnexus', command: 'cypher', ms: 100, stdoutBytes: 2 })
    })
  })
})
