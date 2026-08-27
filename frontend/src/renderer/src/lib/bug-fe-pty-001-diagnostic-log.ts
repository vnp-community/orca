// TEMP diagnostic-only module for the BUG-FE-PTY-001 investigation. Delete
// this file (and its call sites) once the investigation closes.
//
// Why localStorage instead of plain console.error: live repros have
// repeatedly lost or truncated DevTools console output in the field (a
// panel clear, virtualization, or the user only copying the last visible
// line) — a persisted ring buffer survives all of that and can be dumped as
// one paste-able string on demand via `orcaDumpBugFePty001()` in the
// console, immune to how the console itself was captured.

const STORAGE_KEY = 'orca.diag.bugFePty001'
const DIAG_LIMIT = 300

type DiagEntry = {
  t: number
  msg: string
}

// Why typeof window guard: this module is imported transitively by
// worktrees.ts/web-session-tabs-sync.ts, which unit tests import under a
// node (non-jsdom) environment — a bare `window` reference at module-eval
// time would throw ReferenceError before any test body runs.
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

export function logBugFePty001(message: string): void {
  // eslint-disable-next-line no-console -- intentional, temporary diagnostic
  console.error(`[DIAG BUG-FE-PTY-001] ${message}`)
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
    orcaDumpBugFePty001?: () => string
    orcaClearBugFePty001?: () => void
  }
}

// Why: exposed on window so a live repro's full diagnostic history can be
// retrieved with one console command instead of manually scrolling/copying.
if (typeof window !== 'undefined') {
  window.orcaDumpBugFePty001 = (): string => {
    const entries = readBuffer()
    const text = entries
      .map((entry) => `[${new Date(entry.t).toISOString()}] ${entry.msg}`)
      .join('\n\n')
    // eslint-disable-next-line no-console -- intentional, temporary diagnostic
    console.log(text)
    return text
  }

  window.orcaClearBugFePty001 = (): void => {
    try {
      window.localStorage.removeItem(STORAGE_KEY)
    } catch {
      // best-effort
    }
  }
}
