export type SymbolRef = {
  uid: string
  kind: string
  name: string
  qualifiedName: string
  filePath: string
  startLine: number | null
  endLine: number | null
  rawKind?: string
  ordinal?: number
  key: string
}

export function canonicalKind(nativeLabel: string): { kind: string; rawKind?: string; unknown?: boolean } {
  const l = nativeLabel.toLowerCase()
  switch (l) {
    case 'function': return { kind: 'function' }
    case 'method': case 'constructor': return { kind: 'method' }
    case 'struct': case 'class': case 'interface': case 'enum': return { kind: 'type', rawKind: nativeLabel }
    case 'const': case 'variable': case 'property': return { kind: 'value' }
    case 'file': return { kind: 'file' }
    case 'folder': return { kind: 'folder' }
    case 'route': return { kind: 'route' }
    case 'community': return { kind: 'cluster' }
    case 'process': return { kind: 'flow' }
    case 'section': return { kind: 'doc' }
    // generic fallback
    default: return { kind: 'value', rawKind: nativeLabel, unknown: true }
  }
}

export function parseGitNexusId(id: string): { label: string; filePath: string; qualified: string; ordinal?: number } {
  // Label:filePath:qualified#ordinal
  const firstColon = id.indexOf(':')
  if (firstColon === -1) return { label: 'Unknown', filePath: '', qualified: id }
  const label = id.substring(0, firstColon)
  const rest = id.substring(firstColon + 1)
  
  const secondColon = rest.indexOf(':')
  if (secondColon === -1) return { label, filePath: rest, qualified: rest }
  const filePath = rest.substring(0, secondColon)
  let qualifiedWithOrdinal = rest.substring(secondColon + 1)

  let ordinal: number | undefined
  const hashIndex = qualifiedWithOrdinal.lastIndexOf('#')
  if (hashIndex !== -1) {
    const afterHash = qualifiedWithOrdinal.substring(hashIndex + 1)
    if (/^\d+$/.test(afterHash)) {
      ordinal = parseInt(afterHash, 10)
      qualifiedWithOrdinal = qualifiedWithOrdinal.substring(0, hashIndex)
    }
  }

  return { label, filePath, qualified: qualifiedWithOrdinal, ordinal }
}

export function buildSymbolRef(raw: { id: string; label: string; name: string; filePath?: string; startLine?: number | null; endLine?: number | null }, warnings: string[] = []): SymbolRef {
  const { kind, rawKind, unknown } = canonicalKind(raw.label)
  if (unknown) {
    warnings.push('unknown_native_kind')
  }

  let filePath = raw.filePath || ''
  let qualifiedName = raw.name || ''
  let ordinal: number | undefined

  if (raw.id.includes(':')) {
    const parsed = parseGitNexusId(raw.id)
    filePath = parsed.filePath || filePath
    qualifiedName = parsed.qualified || qualifiedName
    ordinal = parsed.ordinal
  }

  // normalize qualified name
  qualifiedName = qualifiedName.normalize('NFC').replace(/::/g, '.')

  let key = ''
  if (kind === 'cluster' || kind === 'flow') {
    key = `${kind}::${raw.id}`
  } else if (kind === 'file' || kind === 'folder') {
    key = `${kind}:${filePath}:${raw.name}`
  } else if (kind === 'doc') {
    // Section:CLAUDE.md:L23:GitNexus
    key = `${kind}:${filePath}:L${(raw.startLine || 0) + 1}:${raw.name}`
  } else {
    key = `${kind}:${filePath}:${qualifiedName}`
  }

  return {
    uid: raw.id,
    kind,
    name: raw.name,
    qualifiedName,
    filePath,
    startLine: typeof raw.startLine === 'number' ? raw.startLine + 1 : null,
    endLine: typeof raw.endLine === 'number' ? raw.endLine + 1 : null,
    ...(rawKind && { rawKind }),
    ...(ordinal !== undefined && { ordinal }),
    key
  }
}

export function assignUniqueKeys(refs: SymbolRef[], warnings: string[]): void {
  const seen = new Map<string, number>()
  for (const ref of refs) {
    let key = ref.key
    let count = seen.get(key) || 0
    if (count > 0) {
      warnings.push('key_collision')
      // arity/ordinal fallback, just use L<startLine>
      if (ref.startLine) {
        key = `${key}#L${ref.startLine}`
      } else {
        key = `${key}#${count}`
      }
      ref.key = key
    }
    // Update map with new key to avoid chaining collision
    seen.set(key, 1)
    seen.set(ref.key.split('#')[0], count + 1)
  }
}
