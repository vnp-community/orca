import { SymbolRef } from './codeintel-symbol-ref'

export type CodeGraphSymbolRef = SymbolRef & {
  codegraphId?: string
  signature?: string
  docstring?: string
  isExported?: boolean
}

export function canonicalCodeGraphKind(cgType: string): { kind: string; rawKind?: string; drop?: boolean } {
  switch (cgType.toLowerCase()) {
    case 'function': return { kind: 'function' }
    case 'method': return { kind: 'method' }
    case 'struct': case 'class': case 'interface': case 'enum': case 'type_alias': return { kind: 'type', rawKind: cgType }
    case 'constant': case 'variable': case 'property': case 'field': case 'enum_member': return { kind: 'value', rawKind: cgType }
    case 'file': return { kind: 'file' }
    case 'route': return { kind: 'route' }
    case 'component': return { kind: 'component', rawKind: cgType } // Should component have rawKind component? Contract says route, component, namespace.
    case 'namespace': return { kind: 'namespace' }
    case 'import': return { kind: 'value', drop: true }
    default: return { kind: 'value', rawKind: cgType }
  }
}

export function buildSymbolRefFromCodeGraph(node: any, warnings: string[] = []): CodeGraphSymbolRef | null {
  const typeLabel = typeof node.type === 'string' ? node.type : 'unknown'
  const { kind, rawKind, drop } = canonicalCodeGraphKind(typeLabel)
  if (drop) return null

  let qualifiedName = typeof node.name === 'string' ? node.name : ''
  qualifiedName = qualifiedName.replace(/::/g, '.')

  const filePath = typeof node.path === 'string' ? node.path : ''
  const key = `${kind}:${filePath}:${qualifiedName}`

  const name = qualifiedName.split('.').pop() || qualifiedName

  const ref: CodeGraphSymbolRef = {
    uid: node.id ? String(node.id) : '',
    kind,
    name,
    qualifiedName,
    filePath,
    startLine: typeof node.start_line === 'number' ? node.start_line : null,
    endLine: typeof node.end_line === 'number' ? node.end_line : null,
    key
  }

  if (rawKind) ref.rawKind = rawKind
  if (node.id !== undefined && node.id !== null) ref.codegraphId = String(node.id)
  
  if (typeof node.signature === 'string' && node.signature) {
    ref.signature = node.signature
  }
  if (typeof node.docstring === 'string' && node.docstring) {
    ref.docstring = node.docstring.substring(0, 4096)
  }
  if (typeof node.is_exported === 'boolean') {
    ref.isExported = node.is_exported
  }

  return ref
}

export function detectSourcesDisagree(a: SymbolRef, b: SymbolRef): boolean {
  if (a.key !== b.key) return false
  if (a.startLine === null || b.startLine === null) return false
  return Math.abs(a.startLine - b.startLine) > 2
}
