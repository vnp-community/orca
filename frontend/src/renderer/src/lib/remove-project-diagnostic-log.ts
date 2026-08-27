// TEMP diagnostic-only module for the "Remove Project" no-op investigation.
// Delete this file (and its call sites) once the investigation closes.
//
// Why localStorage instead of plain console.error: the reporter saw zero
// console output at all around a repro, which console.error alone can't
// help diagnose (a filtered log level, a different tab, or the message
// simply not scrolled into view all look identical to "nothing happened").
// A persisted ring buffer survives all of that and can be dumped as one
// paste-able string on demand via `orcaDumpRemoveProjectDiagnostic()`.

const STORAGE_KEY = 'orca.diag.removeProject'
const DIAG_LIMIT = 100

type DiagEntry = {
  t: number
  msg: string
}

function readBuffer(): DiagEntry[] {
  if (typeof window === 'undefined') {
    return []
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as DiagEntry[]) : []
  } catch {
    return []
  }
}

function writeBuffer(entries: DiagEntry[]): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(entries))
  } catch {
    // Why: quota/private-mode failures must not break the app — this
    // diagnostic is best-effort only, never load-bearing.
  }
}

export function logRemoveProjectDiagnostic(message: string): void {
  // eslint-disable-next-line no-console -- intentional, temporary diagnostic
  console.error(`[DIAG remove-project] ${message}`)
  const entries = readBuffer()
  entries.push({ t: Date.now(), msg: message })
  if (entries.length > DIAG_LIMIT) {
    entries.splice(0, entries.length - DIAG_LIMIT)
  }
  writeBuffer(entries)
}

declare global {
  // oxlint-disable-next-line typescript-eslint/consistent-type-definitions -- declaration merging requires interface
  interface Window {
    orcaDumpRemoveProjectDiagnostic?: () => string
    orcaClearRemoveProjectDiagnostic?: () => void
  }
}

if (typeof window !== 'undefined') {
  window.orcaDumpRemoveProjectDiagnostic = (): string => {
    const entries = readBuffer()
    const text = entries
      .map((entry) => `[${new Date(entry.t).toISOString()}] ${entry.msg}`)
      .join('\n\n')
    // eslint-disable-next-line no-console -- intentional, temporary diagnostic
    console.log(text)
    return text
  }

  window.orcaClearRemoveProjectDiagnostic = (): void => {
    try {
      window.localStorage.removeItem(STORAGE_KEY)
    } catch {
      // best-effort
    }
  }
}
