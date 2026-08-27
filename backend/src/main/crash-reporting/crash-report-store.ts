import crypto from 'node:crypto'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { getCanonicalUserDataPath } from '../persistence-paths'
import { grantDirAclAsync, isPermissionError } from '../win32-utils'
import {
  formatCrashReportText,
  sanitizeCrashReportBreadcrumbs,
  sanitizeCrashReportDetails,
  type CrashReportCreateInput,
  type CrashReportRecord,
  type CrashReportStatus
} from '../../shared/crash-reporting'

// Why: server mode has no Electron crashReporter/native dump capture (see
// desktop/src/main/crash-reporting/crash-report-store.ts's header comment for
// the Electron-only capture path this mirrors). This store instead persists
// only what is explicitly reported to it — React error-boundary submissions
// from the browser client (crashReports.recordRendererError) plus best-effort
// process.on('uncaughtException')/'unhandledRejection' captures wired in
// server-bootstrap.ts. Same on-disk shape/limits as desktop so the shared
// frontend UI (CrashReportRecord, formatCrashReportText) needs no branching.
const MAX_REPORTS = 5
const RELATED_CRASH_WINDOW_MS = 5_000
const WINDOWS_FILE_OPERATION_RETRY_DELAYS_MS = [50, 100, 150, 200, 250]

type CrashReportFile = {
  reports: CrashReportRecord[]
}

function isRelatedCrashEvent(anchor: CrashReportRecord, candidate: CrashReportRecord): boolean {
  if (anchor.id === candidate.id || candidate.status !== 'pending') {
    return false
  }
  const anchorTime = Date.parse(anchor.createdAt)
  const candidateTime = Date.parse(candidate.createdAt)
  if (!Number.isFinite(anchorTime) || !Number.isFinite(candidateTime)) {
    return false
  }
  return (
    Math.abs(anchorTime - candidateTime) <= RELATED_CRASH_WINDOW_MS &&
    anchor.reason === candidate.reason &&
    anchor.exitCode === candidate.exitCode &&
    anchor.appVersion === candidate.appVersion &&
    anchor.platform === candidate.platform
  )
}

function isRetryableWindowsFileOperationError(error: unknown): boolean {
  const code = (error as NodeJS.ErrnoException).code
  return code === 'EPERM' || code === 'EACCES' || code === 'EBUSY'
}

async function wait(delayMs: number): Promise<void> {
  await new Promise<void>((resolve) => setTimeout(resolve, delayMs))
}

async function runCrashReportFileOperationWithWindowsRecovery<T>(
  directory: string,
  operation: () => Promise<T>
): Promise<T> {
  let repairedAcl = false
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await operation()
    } catch (error) {
      if (
        process.platform !== 'win32' ||
        !isRetryableWindowsFileOperationError(error) ||
        attempt >= WINDOWS_FILE_OPERATION_RETRY_DELAYS_MS.length
      ) {
        throw error
      }
      if (!repairedAcl && isPermissionError(error)) {
        repairedAcl = true
        try {
          await grantDirAclAsync(directory)
        } catch {
          // The bounded retry below still handles transient file locks.
        }
      }
      await wait(WINDOWS_FILE_OPERATION_RETRY_DELAYS_MS[attempt])
    }
  }
}

export class CrashReportStore {
  private writeChain = Promise.resolve()

  constructor(private readonly filePath: string) {}

  static fromUserData(userDataPath = getCanonicalUserDataPath()): CrashReportStore {
    return new CrashReportStore(path.join(userDataPath, 'crash-reports.json'))
  }

  async record(input: CrashReportCreateInput): Promise<CrashReportRecord> {
    return this.withWrite(async (reports) => {
      const report: CrashReportRecord = {
        ...input,
        id: crypto.randomUUID(),
        createdAt: new Date().toISOString(),
        status: 'pending',
        details: sanitizeCrashReportDetails(input.details),
        breadcrumbs: sanitizeCrashReportBreadcrumbs(input.breadcrumbs)
      }
      return {
        reports: [report, ...reports].slice(0, MAX_REPORTS),
        result: report
      }
    })
  }

  async getLatestPending(): Promise<CrashReportRecord | null> {
    const reports = await this.readReports()
    return reports.find((report) => report.status === 'pending') ?? null
  }

  async listRecent(): Promise<CrashReportRecord[]> {
    return this.readReports()
  }

  async markSent(id: string): Promise<CrashReportRecord | null> {
    return this.transitionPending(id, 'sent')
  }

  async markDismissedSent(id: string): Promise<CrashReportRecord | null> {
    return this.transitionStatus(id, 'dismissed', 'sent')
  }

  async dismiss(id: string): Promise<CrashReportRecord | null> {
    return this.transitionPending(id, 'dismissed')
  }

  async formatDiagnosticText(id: string, notes?: string): Promise<string | null> {
    const reports = await this.readReports()
    const report = reports.find((candidate) => candidate.id === id)
    return report ? formatCrashReportText(report, notes) : null
  }

  async getById(id: string): Promise<CrashReportRecord | null> {
    const reports = await this.readReports()
    return reports.find((report) => report.id === id) ?? null
  }

  private async transitionPending(
    id: string,
    status: Exclude<CrashReportStatus, 'pending'>
  ): Promise<CrashReportRecord | null> {
    return this.transitionStatus(id, 'pending', status)
  }

  private async transitionStatus(
    id: string,
    from: CrashReportStatus,
    status: Exclude<CrashReportStatus, 'pending'>
  ): Promise<CrashReportRecord | null> {
    return this.withWrite(async (reports) => {
      let result: CrashReportRecord | null = null
      const anchor = reports.find((report) => report.id === id)
      const nextReports = reports.map((report) => {
        if (report.id !== id) {
          // Why: one crash burst can emit several related reports (e.g. an
          // uncaughtException followed by an unhandledRejection during
          // teardown). Once one is handled, treat siblings as already covered.
          if (anchor && anchor.status === from && isRelatedCrashEvent(anchor, report)) {
            return { ...report, status: 'dismissed' as const }
          }
          return report
        }
        if (report.status !== from) {
          result = report
          return report
        }
        result = { ...report, status }
        return result
      })
      return { reports: nextReports, result }
    })
  }

  private async withWrite<T>(
    mutate: (reports: CrashReportRecord[]) => Promise<{ reports: CrashReportRecord[]; result: T }>
  ): Promise<T> {
    const run = this.writeChain.then(async () => {
      // Why: awaiting writeChain from inside its own callback would deadlock;
      // this writer already has exclusive ownership and can read disk directly.
      const reports = await this.readReportsFromDisk()
      const { reports: nextReports, result } = await mutate(reports)
      await this.writeReports(nextReports)
      return result
    })
    this.writeChain = run.then(
      () => undefined,
      () => undefined
    )
    return run
  }

  private async readReports(): Promise<CrashReportRecord[]> {
    // Why: a concurrent caller can query while a record()/dismiss() write is
    // still in flight. Wait so a just-persisted change is visible.
    await this.writeChain
    return this.readReportsFromDisk()
  }

  private async readReportsFromDisk(): Promise<CrashReportRecord[]> {
    try {
      const raw = await runCrashReportFileOperationWithWindowsRecovery(
        path.dirname(this.filePath),
        () => fs.readFile(this.filePath, 'utf8')
      )
      const parsed = JSON.parse(raw) as Partial<CrashReportFile>
      return Array.isArray(parsed.reports) ? parsed.reports.slice(0, MAX_REPORTS) : []
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT') {
        console.warn('[crash-reporting] Failed to read crash reports:', error)
      }
      return []
    }
  }

  private async writeReports(reports: CrashReportRecord[]): Promise<void> {
    const directory = path.dirname(this.filePath)
    const tmpPath = `${this.filePath}.${process.pid}.${Date.now()}.${crypto.randomUUID()}.tmp`
    try {
      await runCrashReportFileOperationWithWindowsRecovery(directory, async () => {
        await fs.mkdir(directory, { recursive: true })
        await fs.writeFile(tmpPath, `${JSON.stringify({ reports }, null, 2)}${os.EOL}`, 'utf8')
        await fs.rename(tmpPath, this.filePath)
      })
    } finally {
      // Why: disk-full and terminal rename failures must not accumulate a new
      // orphaned multi-report temp file after every crash.
      await fs.rm(tmpPath, { force: true }).catch(() => {})
    }
  }
}

let sharedStore: CrashReportStore | null = null

// Why: the RPC methods (runtime/rpc/methods/crash-reports.ts) and the
// process-level handlers below both need the exact same store instance so a
// crash captured by process.on() is visible to a subsequent
// crashReports.getLatestPending call — mirrors desktop's
// getCrashReportStoreForRpc()/registerCrashReportingHandlers pairing.
export function getSharedCrashReportStore(): CrashReportStore {
  if (!sharedStore) {
    sharedStore = CrashReportStore.fromUserData()
  }
  return sharedStore
}

export function setSharedCrashReportStoreForTest(store: CrashReportStore | null): void {
  sharedStore = store
}

let processHandlersInstalled = false

// Why: shared by the RPC handlers (crashReports.recordRendererError,
// crashReports.submit, crashReports.copyLatestDiagnostics) so every report —
// process-captured or explicitly submitted — carries the same host context.
export function serverCrashReportContext(): Omit<
  CrashReportCreateInput,
  'source' | 'processType' | 'reason' | 'exitCode' | 'details'
> {
  return {
    appVersion: process.env.npm_package_version ?? 'unknown',
    platform: process.platform,
    osRelease: os.release(),
    arch: process.arch,
    // Why: these fields exist for on-disk/UI parity with the Electron-desktop
    // shape (formatCrashReportText prints them unconditionally) — server mode
    // has neither an Electron runtime nor a Chrome renderer.
    electronVersion: 'n/a (server mode)',
    chromeVersion: 'n/a (server mode)'
  }
}

// Why: crashReports.submit / copyLatestDiagnostics fall back to an
// "uncaptured" report (formatUncapturedCrashReportText) when no stored
// CrashReportRecord matches — mirrors desktop's buildUncapturedCrashReportText.
export function buildServerUncapturedCrashReportContext(): {
  createdAt: string
  appVersion: string
  platform: NodeJS.Platform
  osRelease: string
  arch: string
  electronVersion: string
  chromeVersion: string
} {
  return {
    createdAt: new Date().toISOString(),
    ...serverCrashReportContext()
  }
}

function recordServerProcessCrash(reason: string, error: unknown): void {
  const store = getSharedCrashReportStore()
  void store
    .record({
      ...serverCrashReportContext(),
      source: 'child',
      processType: 'node-server',
      reason,
      exitCode: null,
      details: {
        error_name: error instanceof Error ? error.name : typeof error,
        error_message: error instanceof Error ? error.message : String(error),
        ...(error instanceof Error && error.stack ? { error_stack: error.stack } : {})
      }
    })
    .catch((persistError) => {
      console.error('[crash-reporting] Failed to persist server crash report:', persistError)
    })
}

// Why: server mode has no Electron crashReporter to catch a fatal main-process
// error. Best-effort persist a report (so the next crashReports.getLatestPending
// call can surface it), then preserve Node's default fatal behavior — an
// unhandled error must still take the process down for the process supervisor
// (systemd/pm2/container orchestrator) to restart it; silently swallowing it
// here would leave the server running in a corrupted state instead.
export function installServerCrashReportProcessHandlers(): void {
  if (processHandlersInstalled) {
    return
  }
  processHandlersInstalled = true

  process.on('uncaughtException', (error) => {
    console.error('[crash-reporting] Uncaught exception:', error)
    recordServerProcessCrash('uncaught-exception', error)
    // Why: give the fire-and-forget write a brief window to land on disk
    // before we take the process down, without blocking shutdown indefinitely.
    setTimeout(() => process.exit(1), 250).unref()
  })

  process.on('unhandledRejection', (reason) => {
    console.error('[crash-reporting] Unhandled rejection:', reason)
    recordServerProcessCrash('unhandled-rejection', reason)
    setTimeout(() => process.exit(1), 250).unref()
  })
}

export function _resetCrashReportProcessHandlersForTest(): void {
  processHandlersInstalled = false
}
