/**
 * Solution content sections — CR-REQ-020-02
 *
 * The wire parser keeps AI output as a plain `content` string. Diagnosis,
 * findings and answer views read named sections out of it when it is a JSON
 * object (snake_case or camelCase keys), else fall back to showing it whole.
 *
 * @module components/request/solution/solution-content-sections
 */

export type SectionSpec = { name: string; aliases: string[] }

export type ContentSections = {
  /** Section name -> text lines; missing sections are absent. */
  sections: Record<string, string[]>
  /** Raw text when nothing structured was found. */
  fallbackText: string | null
}

function toLines(value: unknown): string[] {
  if (value === null || value === undefined) {return []}
  if (typeof value === 'string') {return value.trim() ? [value] : []}
  if (typeof value === 'number' || typeof value === 'boolean') {return [String(value)]}
  if (Array.isArray(value)) {return value.flatMap(toLines)}
  if (typeof value === 'object') {
    const r = value as Record<string, unknown>
    const text = ['title', 'text', 'summary', 'name', 'ref', 'url'].map((k) => r[k]).filter((v) => typeof v === 'string' && v)
    return text.length > 0 ? [(text as string[]).join(' - ')] : []
  }
  return []
}

export function readContentSections(content: string | undefined, specs: SectionSpec[]): ContentSections {
  const text = content ?? ''
  const trimmed = text.trim()
  if (!trimmed) {return { sections: {}, fallbackText: null }}
  if (trimmed.startsWith('{')) {
    try {
      const parsed: unknown = JSON.parse(trimmed)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        const obj = parsed as Record<string, unknown>
        const sections: Record<string, string[]> = {}
        for (const spec of specs) {
          for (const alias of spec.aliases) {
            const lines = toLines(obj[alias])
            if (lines.length > 0) {
              sections[spec.name] = lines
              break
            }
          }
        }
        if (Object.keys(sections).length > 0) {return { sections, fallbackText: null }}
      }
    } catch {
      // Not JSON: render as text below.
    }
  }
  return { sections: {}, fallbackText: text }
}
