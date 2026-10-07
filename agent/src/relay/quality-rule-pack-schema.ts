export type RuleKind = 'diff' | 'script' | 'profile-ref'

export type Rule = {
  id: string
  title: string
  kind: RuleKind
  category: 'convention'
  severity: 'error' | 'warning' | 'info'
  enabled: boolean
  scope: {
    include: string[]
    exclude: string[]
    fileStatus: ('added' | 'modified' | 'renamed')[]
  }
  match?: {
    type: 'added-line-regex' | 'added-file-name' | 'file-content-regex'
    pattern: string
    maxLineLength: number
  }
  script?: {
    name: string
    locate: string[]
    cwd: 'repoRoot' | 'desktop'
    args: string[] // Wait, the spec says "args: []", but TS array of strings is fine if we check it in validation.
  }
  ref?: {
    profileId: string
  }
  message: string
  fixHint: string
  source: {
    doc: string
    script?: string
  }
}

export type RulePack = {
  version: number
  pack: string
  rules: Rule[]
}

const RESERVED_IDS = new Set(['ORCA-008', 'ORCA-009', 'ORCA-016', 'ORCA-017'])
const ORCA_ID_REGEX = /^ORCA-\d{3}$/

export function validateRulePack(raw: any): RulePack {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('RulePack must be an object')
  }

  if (raw.version !== 1) {
    throw new Error('RulePack version must be 1')
  }

  if (!Array.isArray(raw.rules)) {
    throw new Error('RulePack rules must be an array')
  }

  const ids = new Set<string>()

  for (const rule of raw.rules) {
    if (!rule.id || !ORCA_ID_REGEX.test(rule.id)) {
      throw new Error(`Invalid rule id format: ${rule.id}`)
    }

    if (RESERVED_IDS.has(rule.id)) {
      throw new Error(`Rule id is reserved: ${rule.id}`)
    }

    if (ids.has(rule.id)) {
      throw new Error(`Duplicate rule id: ${rule.id}`)
    }
    ids.add(rule.id)

    if (rule.match) {
      if (rule.match.maxLineLength < 1 || rule.match.maxLineLength > 2000) {
        throw new Error(`Rule ${rule.id} maxLineLength out of bounds`)
      }

      // Check safe regex subset (heuristics)
      const pattern = rule.match.pattern
      if (pattern.includes('(?<=') || pattern.includes('(?<!')) {
        throw new Error(`Rule ${rule.id} regex contains lookbehind`)
      }
      if (pattern.match(/\([^)]*\+\)\+/)) {
        throw new Error(`Rule ${rule.id} regex contains nested repetitions`)
      }
      if (pattern.match(/\{\d+,(\d+)?\}.*\{\d+,(\d+)?\}/)) {
        // very rough heuristic for nested curly braces
        // wait, we only want to reject "nhóm lặp lồng `(a+)+`, `{n,}` lồng".
        if (pattern.match(/\([^)]*\{\d+,?\}.*\)\{\d+,?\}/)) {
           throw new Error(`Rule ${rule.id} regex contains nested repetitions`)
        }
      }

      try {
        new RegExp(pattern)
      } catch (e) {
        throw new Error(`Rule ${rule.id} invalid regex: ${pattern}`)
      }
    }

    if (rule.script) {
      if (!Array.isArray(rule.script.args) || rule.script.args.length > 0) {
        throw new Error(`Rule ${rule.id} script args must be empty array`)
      }
    }

    if (rule.kind === 'profile-ref') {
      if (!rule.ref || !rule.ref.profileId) {
        throw new Error(`Rule ${rule.id} profile-ref must have ref.profileId`)
      }
    }
  }

  return raw as RulePack
}
