import { CodeIntelError } from './codeintel-errors'

const FORBIDDEN_WORDS = [
  'CREATE', 'MERGE', 'DELETE', 'SET', 'REMOVE', 'DROP', 'ALTER', 'COPY',
  'DETACH', 'CALL', 'LOAD', 'INSTALL', 'ATTACH', 'EXPORT', 'IMPORT',
  'FOREACH', 'UNWIND'
]

const FORBIDDEN_REGEX = new RegExp(`\\b(${FORBIDDEN_WORDS.join('|')})\\b`, 'i')

export function assertReadOnlyCypher(text: string): string {
  if (Buffer.byteLength(text, 'utf8') > 16384) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Cypher query exceeds 16KiB')
  }
  if (!text.trimStart().startsWith('MATCH ')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Cypher query must start with MATCH')
  }

  let i = 0
  let inString = false
  let stripped = ''

  while (i < text.length) {
    const c = text[i]
    if (inString) {
      if (c === '\\') {
        i += 2
        continue
      } else if (c === "'") {
        inString = false
      }
    } else {
      if (c === "'") {
        inString = true
      } else {
        stripped += c
      }
    }
    i++
  }

  if (inString) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Unclosed string literal in Cypher query')
  }

  if (stripped.includes(';')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Multiple statements are not allowed')
  }

  const match = stripped.match(FORBIDDEN_REGEX)
  if (match) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Forbidden Cypher keyword: ${match[1]}`)
  }

  return text
}
