/**
 * DevServerFilesystemProvider — IFilesystemProvider backed by a Dev Server's
 * existing agent-WebSocket connection (see dev-server-provider-lifecycle.ts).
 *
 * Unlike SshFilesystemProvider (a thin 1:1 forwarder to the SSH relay
 * daemon's rich fs.* surface), the dev-server agent only exposes a narrow
 * set of methods (src/relay/fs-agent-extensions.ts): fs.stat, fs.readDir,
 * fs.readFile, fs.writeFile, fs.mkdir, fs.rmdir, fs.glob, fs.grep. Every
 * method below composes from that surface; methods with no agent
 * equivalent at all throw a clear "not supported" error instead of
 * silently misbehaving. All other IFilesystemProvider methods
 * (readTerminalArtifact, downloadFile, openFileUploadSession, getTempDir,
 * writeTerminalArtifact, lstat, scanWorkspaceSpace) are optional on the
 * interface and intentionally omitted.
 */
import type { DirEntry, FsChangeEvent, SearchOptions, SearchResult } from '../../shared/types'
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import type { FileReadResult, FileStat, IFilesystemProvider } from './types'

const NOT_SUPPORTED = (op: string): Error =>
  new Error(`${op} is not supported for Dev Server hosts yet.`)

/** Matches the top-level shape of fs.readDir's recursive result (fs-agent-extensions.ts). */
type AgentFileTreeNode = {
  name: string
  type: 'file' | 'directory'
}

type AgentStatResult = {
  size: number
  mtime: string
  isDir: boolean
  isFile: boolean
  isLink: boolean
}

/** Poll interval for the fallback file-watch implementation (Phase 1 — see plan). */
const WATCH_POLL_INTERVAL_MS = 3_000

export class DevServerFilesystemProvider implements IFilesystemProvider {
  constructor(
    private readonly devServerId: string,
    private readonly relay: DevServerRelayConnection
  ) {}

  async readDir(dirPath: string): Promise<DirEntry[]> {
    const result = await this.relay.call<{ entries: AgentFileTreeNode[] }>('fs.readDir', {
      path: dirPath,
      depth: 1
    })
    return result.entries.map((entry) => ({
      name: entry.name,
      isDirectory: entry.type === 'directory',
      // Why: fs.readDir doesn't distinguish symlinks from regular entries —
      // acceptable v1 approximation, matches this provider's narrow surface.
      isSymlink: false
    }))
  }

  async readFile(filePath: string): Promise<FileReadResult> {
    const result = await this.relay.call<{
      content: string
      encoding: 'utf-8' | 'base64'
      isBinary: boolean
    }>('fs.readFile', { path: filePath })
    return { content: result.content, isBinary: result.isBinary }
  }

  async writeFile(filePath: string, content: string): Promise<void> {
    await this.relay.call('fs.writeFile', { path: filePath, content })
  }

  async writeFileBase64(filePath: string, contentBase64: string): Promise<void> {
    // Why: the agent's fs.writeFile accepts an `encoding` param and writes
    // via Node's fs.writeFile(path, content, encoding) — passing 'base64'
    // decodes the string into raw bytes before writing, so this round-trips
    // binary content correctly without a separate upload path.
    await this.relay.call('fs.writeFile', { path: filePath, content: contentBase64, encoding: 'base64' })
  }

  async writeFileBase64Chunk(filePath: string, contentBase64: string, append: boolean): Promise<void> {
    if (append) {
      throw NOT_SUPPORTED('Chunked/append binary upload')
    }
    await this.writeFileBase64(filePath, contentBase64)
  }

  async stat(filePath: string): Promise<FileStat> {
    const result = await this.relay.call<AgentStatResult>('fs.stat', { path: filePath })
    return {
      size: result.size,
      type: result.isDir ? 'directory' : result.isLink ? 'symlink' : 'file',
      mtime: Date.parse(result.mtime)
    }
  }

  async deletePath(targetPath: string, recursive?: boolean): Promise<void> {
    await this.relay.call('fs.rmdir', { path: targetPath, recursive: recursive === true })
  }

  async createFile(filePath: string): Promise<void> {
    await this.relay.call('fs.writeFile', { path: filePath, content: '' })
  }

  async createDir(dirPath: string): Promise<void> {
    // Why: the agent's fs.mkdir is always recursive (mkdir(path, {recursive:true})).
    await this.relay.call('fs.mkdir', { path: dirPath })
  }

  async createDirNoClobber(dirPath: string): Promise<void> {
    // Why: the agent has no atomic no-clobber mkdir. This stat-then-create
    // has a race window; acceptable for v1 since it's a secondary flow
    // (see plan's provider-scoping notes), but not a correctness guarantee.
    const exists = await this.stat(dirPath).then(
      () => true,
      () => false
    )
    if (exists) {
      throw new Error(`Directory already exists: ${dirPath}`)
    }
    await this.createDir(dirPath)
  }

  async rename(): Promise<void> {
    throw NOT_SUPPORTED('Rename')
  }

  async renameNoClobber(): Promise<void> {
    throw NOT_SUPPORTED('Rename')
  }

  async copy(): Promise<void> {
    throw NOT_SUPPORTED('Copy')
  }

  async realpath(): Promise<string> {
    throw NOT_SUPPORTED('Resolving real paths')
  }

  async search(opts: SearchOptions): Promise<SearchResult> {
    const result = await this.relay.call<{
      matches: { file: string; line: number; text: string }[]
      truncated: boolean
    }>('fs.grep', {
      root: opts.rootPath,
      pattern: opts.query,
      maxResults: opts.maxResults ?? 200
    })
    const byFile = new Map<string, { line: number; text: string }[]>()
    for (const match of result.matches) {
      const list = byFile.get(match.file) ?? []
      list.push({ line: match.line, text: match.text })
      byFile.set(match.file, list)
    }
    const files = Array.from(byFile.entries()).map(([filePath, matches]) => ({
      filePath,
      relativePath: filePath,
      matches: matches.map((m) => ({
        line: m.line,
        // Why: fs.grep doesn't report a match column/length — approximate
        // with the whole-line span until the agent protocol carries it.
        column: 0,
        matchLength: opts.query.length,
        lineContent: m.text
      }))
    }))
    return {
      files,
      totalMatches: result.matches.length,
      truncated: result.truncated
    }
  }

  async listFiles(rootPath: string, options?: { excludePaths?: string[] }): Promise<string[]> {
    const result = await this.relay.call<{ paths: string[] }>('fs.glob', {
      pattern: '*',
      cwd: rootPath
    })
    const exclude = options?.excludePaths ?? []
    if (exclude.length === 0) {
      return result.paths
    }
    return result.paths.filter((p) => !exclude.some((ex) => p === ex || p.startsWith(`${ex}/`)))
  }

  /**
   * Phase 3: pushes real fs.changed notifications when the agent supports
   * fs.watch (relay.onNotification is present and the call doesn't throw
   * MethodNotFound). Falls back to Phase 1's readDir-diff polling for older
   * agent binaries — a Dev Server rolled out before this feature, or one
   * whose fs.watch call otherwise fails.
   */
  async watch(
    rootPath: string,
    callback: (events: FsChangeEvent[]) => void,
    options?: { signal?: AbortSignal }
  ): Promise<() => void> {
    if (this.relay.onNotification) {
      const stop = await this.watchViaPush(rootPath, callback, options)
      if (stop) {
        return stop
      }
    }
    return this.watchViaPolling(rootPath, callback, options)
  }

  /**
   * Returns null (instead of throwing) when the agent doesn't understand
   * fs.watch, signaling the caller to fall back to polling.
   */
  private async watchViaPush(
    rootPath: string,
    callback: (events: FsChangeEvent[]) => void,
    options?: { signal?: AbortSignal }
  ): Promise<(() => void) | null> {
    const unsubscribe = this.relay.onNotification!((method, params) => {
      if (method !== 'fs.changed' || params.path !== rootPath) {
        return
      }
      if (params.eventType === 'error') {
        callback([{ kind: 'overflow', absolutePath: rootPath }])
        return
      }
      const filename = typeof params.filename === 'string' ? params.filename : null
      const absolutePath = filename ? `${rootPath}/${filename}` : rootPath
      // Why: Node's fs.watch only reports 'rename' (create/delete/rename, it
      // can't tell which) or 'change' (content) — not the create/delete/update
      // split the relay's parcel-watcher-backed path gives. Callers already
      // treat any FsChangeEvent as "re-read this path", so the coarser kind
      // is a correctness-preserving approximation.
      callback([
        {
          kind: params.eventType === 'rename' ? 'rename' : 'update',
          absolutePath
        }
      ])
    })

    try {
      await this.relay.call('fs.watch', { path: rootPath })
    } catch {
      unsubscribe()
      return null
    }

    const stop = (): void => {
      unsubscribe()
      void this.relay.call('fs.unwatch', { path: rootPath }).catch(() => {})
    }
    options?.signal?.addEventListener('abort', stop, { once: true })
    return stop
  }

  private async watchViaPolling(
    rootPath: string,
    callback: (events: FsChangeEvent[]) => void,
    options?: { signal?: AbortSignal }
  ): Promise<() => void> {
    let previous: Map<string, AgentFileTreeNode> | null = null

    const snapshot = async (): Promise<Map<string, AgentFileTreeNode>> => {
      const result = await this.relay.call<{ entries: AgentFileTreeNode[] }>('fs.readDir', {
        path: rootPath,
        depth: 1
      })
      return new Map(result.entries.map((e) => [e.name, e]))
    }

    const diffAndEmit = async (): Promise<void> => {
      let next: Map<string, AgentFileTreeNode>
      try {
        next = await snapshot()
      } catch {
        return
      }
      if (previous) {
        const events: FsChangeEvent[] = []
        for (const [name, entry] of next) {
          if (!previous.has(name)) {
            events.push({
              kind: 'create',
              absolutePath: `${rootPath}/${name}`,
              isDirectory: entry.type === 'directory'
            })
          }
        }
        for (const [name, entry] of previous) {
          if (!next.has(name)) {
            events.push({
              kind: 'delete',
              absolutePath: `${rootPath}/${name}`,
              isDirectory: entry.type === 'directory'
            })
          }
        }
        if (events.length > 0) {
          callback(events)
        }
      }
      previous = next
    }

    void diffAndEmit()
    const timer = setInterval(() => void diffAndEmit(), WATCH_POLL_INTERVAL_MS)
    const stop = (): void => clearInterval(timer)
    options?.signal?.addEventListener('abort', stop, { once: true })
    return stop
  }

  getConnectionId(): string {
    return this.devServerId
  }
}
