import { describe, it, expect } from 'vitest'
import {
  buildSymbolRefFromCodeGraph,
  detectSourcesDisagree,
  canonicalCodeGraphKind
} from './codeintel-symbol-ref-codegraph'

describe('codeintel-symbol-ref-codegraph', () => {
  it('maps various kinds correctly and drops import', () => {
    expect(canonicalCodeGraphKind('function').kind).toBe('function')
    expect(canonicalCodeGraphKind('class').kind).toBe('type')
    expect(canonicalCodeGraphKind('class').rawKind).toBe('class')
    expect(canonicalCodeGraphKind('variable').kind).toBe('value')
    expect(canonicalCodeGraphKind('import').drop).toBe(true)
    expect(canonicalCodeGraphKind('unknown_type').kind).toBe('value')
  })

  it('builds symbol ref correctly', () => {
    const node = {
      id: 123,
      type: 'method',
      name: 'MyClass::myMethod',
      path: 'src/main.ts',
      start_line: 10,
      end_line: 15,
      signature: '() => void',
      docstring: 'Hello world',
      is_exported: true
    }
    const warnings: string[] = []
    const ref = buildSymbolRefFromCodeGraph(node, warnings)
    
    expect(ref).not.toBeNull()
    if (ref) {
      expect(ref.kind).toBe('method')
      expect(ref.name).toBe('myMethod')
      expect(ref.qualifiedName).toBe('MyClass.myMethod')
      expect(ref.filePath).toBe('src/main.ts')
      expect(ref.key).toBe('method:src/main.ts:MyClass.myMethod')
      expect(ref.startLine).toBe(10)
      expect(ref.codegraphId).toBe('123')
      expect(ref.signature).toBe('() => void')
      expect(ref.docstring).toBe('Hello world')
      expect(ref.isExported).toBe(true)
    }
  })

  it('detects source disagreement', () => {
    const a = { key: 'function:a.ts:foo', startLine: 10 } as any
    const b = { key: 'function:a.ts:foo', startLine: 13 } as any
    expect(detectSourcesDisagree(a, b)).toBe(true)
    
    const c = { key: 'function:a.ts:foo', startLine: 12 } as any
    expect(detectSourcesDisagree(a, c)).toBe(false)
    
    const d = { key: 'different', startLine: 10 } as any
    expect(detectSourcesDisagree(a, d)).toBe(false)
  })
})
