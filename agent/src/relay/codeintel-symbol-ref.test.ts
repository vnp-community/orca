import { describe, it, expect } from 'vitest'
import {
  canonicalKind,
  parseGitNexusId,
  buildSymbolRef,
  assignUniqueKeys
} from './codeintel-symbol-ref'

describe('codeintel-symbol-ref', () => {
  describe('canonicalKind', () => {
    it('maps valid GitNexus labels', () => {
      expect(canonicalKind('Function')).toEqual({ kind: 'function' })
      expect(canonicalKind('Method')).toEqual({ kind: 'method' })
      expect(canonicalKind('Struct')).toEqual({ kind: 'type', rawKind: 'Struct' })
      expect(canonicalKind('Const')).toEqual({ kind: 'value' })
      expect(canonicalKind('Section')).toEqual({ kind: 'doc' })
    })

    it('maps unknown to value with warning', () => {
      expect(canonicalKind('Unknown')).toEqual({ kind: 'value', rawKind: 'Unknown', unknown: true })
    })
  })

  describe('parseGitNexusId', () => {
    it('parses typical symbol id', () => {
      expect(parseGitNexusId('Method:agent/src/relay/context.ts:RelayContext.registerRoot#1'))
        .toEqual({ label: 'Method', filePath: 'agent/src/relay/context.ts', qualified: 'RelayContext.registerRoot', ordinal: 1 })
    })

    it('parses id without ordinal', () => {
      expect(parseGitNexusId('Method:agent/src/relay/context.ts:RelayContext.registerRoot'))
        .toEqual({ label: 'Method', filePath: 'agent/src/relay/context.ts', qualified: 'RelayContext.registerRoot', ordinal: undefined })
    })

    it('parses section id', () => {
      expect(parseGitNexusId('Section:CLAUDE.md:L23:GitNexus — Code Intelligence'))
        .toEqual({ label: 'Section', filePath: 'CLAUDE.md', qualified: 'L23:GitNexus — Code Intelligence', ordinal: undefined })
    })
  })

  describe('buildSymbolRef', () => {
    it('builds method ref correctly and adds 1 to lines', () => {
      const warnings: string[] = []
      const ref = buildSymbolRef({
        id: 'Method:agent/src/relay/context.ts:RelayContext.registerRoot#1',
        label: 'Method',
        name: 'registerRoot',
        filePath: 'agent/src/relay/context.ts',
        startLine: 10,
        endLine: 20
      }, warnings)

      expect(ref.uid).toBe('Method:agent/src/relay/context.ts:RelayContext.registerRoot#1')
      expect(ref.kind).toBe('method')
      expect(ref.startLine).toBe(11)
      expect(ref.endLine).toBe(21)
      expect(ref.qualifiedName).toBe('RelayContext.registerRoot')
      expect(ref.key).toBe('method:agent/src/relay/context.ts:RelayContext.registerRoot')
      expect(warnings).toEqual([])
    })

    it('normalizes :: to . in qualifiedName', () => {
      const ref = buildSymbolRef({
        id: 'Method:src/main.go:RelayContext::registerRoot',
        label: 'Method',
        name: 'RelayContext::registerRoot'
      })
      expect(ref.qualifiedName).toBe('RelayContext.registerRoot')
    })
  })

  describe('assignUniqueKeys', () => {
    it('assigns unique keys when collision occurs', () => {
      const warnings: string[] = []
      const ref1 = buildSymbolRef({
        id: 'Method:foo.ts:handler#1',
        label: 'Method',
        name: 'handler',
        startLine: 10
      })
      const ref2 = buildSymbolRef({
        id: 'Method:foo.ts:handler#2',
        label: 'Method',
        name: 'handler',
        startLine: 20
      })

      assignUniqueKeys([ref1, ref2], warnings)
      expect(ref1.key).toBe('method:foo.ts:handler')
      expect(ref2.key).toBe('method:foo.ts:handler#L21') // 20 + 1
      expect(warnings).toContain('key_collision')
    })
  })
})
