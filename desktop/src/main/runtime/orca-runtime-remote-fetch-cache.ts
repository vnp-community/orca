// frontend/src/main/runtime/orca-runtime-remote-fetch-cache.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-038): git remote-fetch
// dedup/freshness-cache extracted from OrcaRuntimeService. Fully
// self-contained (own state, no other class field/method dependency) —
// confirmed via grep before moving, unlike most domains in this class.
import { gitExecFileAsync } from '../git/runner'
import { GIT_FETCH_SKIP_AUTO_MAINTENANCE_CONFIG_ARGS } from '../../shared/git-fetch-auto-maintenance'
import type { RemoteFetchResult, RemoteTrackingBase } from './orca-runtime-types'

// Why (§3.3): 30s freshness window. A second worktree-create or dispatch-probe
// against the same repo+remote within this window reuses the previous successful
// fetch instead of repeating the round-trip. Chosen so rapid "new worktree"
// clicks and successive coordinator dispatches feel snappy, while still being
// short enough that a genuinely-changed remote is observed on the next action.
const FETCH_FRESHNESS_MS = 30_000
// Why: bound create-path remote fetches so a Windows credential-manager GUI hang
// (STA-1292) can't wedge worktree creation forever; parity with the exact-base
// refresh sibling's timeout.
const REMOTE_FETCH_TIMEOUT_MS = 60_000
const REMOTE_FETCH_CACHE_MAX = 512

function setBoundedMapEntry<K, V>(map: Map<K, V>, key: K, value: V, maxEntries: number): void {
  if (map.has(key)) {
    map.delete(key)
  }
  map.set(key, value)
  while (map.size > maxEntries) {
    const oldest = map.keys().next()
    if (oldest.done) {
      return
    }
    map.delete(oldest.value)
  }
}

// Why (§3.3 + §7.1): the renderer-create path and coordinator
// `probeWorktreeDrift` share this cache so a create that already fetched
// `origin` within the last 30s does not re-fetch during dispatch, and
// vice-versa. Keyed by `<repoPath>::<remote>` so multi-remote repos (even
// though v1 only uses `origin`) don't cross-contaminate. The in-flight Map
// also provides serialization — two concurrent callers share a single
// underlying `git fetch`. Full-remote fetch lifecycle rules:
//   - entry inserted BEFORE await,
//   - `.finally()` removes the entry on BOTH success and rejection,
//   - timestamp written ONLY on success (rejection must not make the
//     30s freshness cache lie).
// A literal "insert before await / read-back after await" without these
// three rules wedges future fetches on the same repo after a single
// DNS hiccup until process restart (see §3.3 Lifecycle). Exact base-ref
// refreshes share the in-flight rule and maintain their own exact-base
// freshness entries; a full-remote fetch may be narrowed by repo refspecs,
// so it must not prove a specific branch for create.
export class RuntimeRemoteFetchCache {
  private readonly fetchInflight = new Map<string, Promise<RemoteFetchResult>>()
  // Why: `git fetch origin` and `git fetch origin <refspec>` contend for the
  // same repo remote/ref locks. This queue serializes all fetch shapes for one
  // canonical repo+remote while still letting same-shape callers share promises.
  private readonly remoteFetchQueueTail = new Map<string, Promise<RemoteFetchResult>>()
  private readonly fetchLastCompletedAt = new Map<string, number>()
  // Why: `getCanonicalFetchKey` is awaited from every freshness probe and
  // every getOrStartRemoteFetch call. Without memoization the warm-cache hot
  // path spawns a `git rev-parse --git-common-dir` subprocess per touch
  // (twice in createLocalWorktree). Cache by `<repoPath>::<remote>` so the
  // canonical key is resolved at most once per repo+remote in the process.
  private readonly canonicalFetchKeyCache = new Map<string, string>()

  async getCanonicalFetchKey(
    repoPath: string,
    remote: string,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<string> {
    const runtimeKey = gitOptions.wslDistro ? `wsl:${gitOptions.wslDistro}` : 'local'
    const cacheKey = `${runtimeKey}::${repoPath}::${remote}`
    const cached = this.canonicalFetchKeyCache.get(cacheKey)
    if (cached !== undefined) {
      setBoundedMapEntry(this.canonicalFetchKeyCache, cacheKey, cached, REMOTE_FETCH_CACHE_MAX)
      return cached
    }
    let resolved = cacheKey
    try {
      const { stdout } = await gitExecFileAsync(
        ['rev-parse', '--path-format=absolute', '--git-common-dir'],
        { cwd: repoPath, ...gitOptions }
      )
      const commonDir = stdout.trim()
      if (commonDir) {
        resolved = `${runtimeKey}::${commonDir}::${remote}`
      }
    } catch {
      // Fall through to the caller-provided path. The fetch still runs from
      // repoPath; this key only controls cache sharing.
    }
    setBoundedMapEntry(this.canonicalFetchKeyCache, cacheKey, resolved, REMOTE_FETCH_CACHE_MAX)
    return resolved
  }

  private enqueueRemoteFetch(
    remoteKey: string,
    runFetch: () => Promise<RemoteFetchResult>
  ): Promise<RemoteFetchResult> {
    const previous = this.remoteFetchQueueTail.get(remoteKey)
    const promise = previous ? previous.then(runFetch, runFetch) : runFetch()
    this.remoteFetchQueueTail.set(remoteKey, promise)
    promise.finally(() => {
      if (this.remoteFetchQueueTail.get(remoteKey) === promise) {
        this.remoteFetchQueueTail.delete(remoteKey)
      }
    })
    return promise
  }

  private getFreshFetchCompletedAt(key: string): number | null {
    const lastAt = this.fetchLastCompletedAt.get(key)
    if (lastAt === undefined) {
      return null
    }
    if (Date.now() - lastAt < FETCH_FRESHNESS_MS) {
      setBoundedMapEntry(this.fetchLastCompletedAt, key, lastAt, REMOTE_FETCH_CACHE_MAX)
      return lastAt
    }
    this.fetchLastCompletedAt.delete(key)
    return null
  }

  private rememberFreshFetchCompletedAt(key: string, completedAt = Date.now()): void {
    setBoundedMapEntry(this.fetchLastCompletedAt, key, completedAt, REMOTE_FETCH_CACHE_MAX)
  }

  async getOrStartRemoteFetch(
    repoPath: string,
    remote: string,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<RemoteFetchResult> {
    const key = await this.getCanonicalFetchKey(repoPath, remote, gitOptions)
    if (this.getFreshFetchCompletedAt(key) !== null) {
      // Why: freshness window hit — skip the fetch entirely. Do NOT reuse any
      // in-flight promise here; the timestamp is only written on success, so
      // hitting this branch means a previous fetch did succeed recently.
      return { ok: true }
    }

    const existing = this.fetchInflight.get(key)
    if (existing) {
      // Why: genuine serialization (not check-then-set). Two callers racing
      // on the same repo+remote share the single underlying `git fetch`.
      return existing
    }

    const promise = this.enqueueRemoteFetch(key, () =>
      gitExecFileAsync(['fetch', remote], {
        cwd: repoPath,
        ...gitOptions,
        // Why: cap the create-path base-ref fetch so a stuck first-auth on
        // Windows (GCM prompt) fails fast instead of hanging creation (STA-1292).
        timeout: REMOTE_FETCH_TIMEOUT_MS
      })
        .then((): RemoteFetchResult => {
          // Why (§3.3 Lifecycle): timestamp on success ONLY. Writing on rejection
          // would make the freshness cache lie about the last known remote state.
          this.rememberFreshFetchCompletedAt(key)
          return { ok: true }
        })
        .catch((err): RemoteFetchResult => {
          // Why: swallow here so awaiters don't throw at the await site. Outer
          // create/dispatch paths are already tolerant of offline fetch failure;
          // this is the behavioral contract of this helper.
          console.warn(`[fetchRemoteWithCache] ${remote} fetch failed for ${repoPath}:`, err)
          return { ok: false, errorKind: 'git_error' }
        })
    ).finally(() => {
      // Why (§3.3 Lifecycle): evict on BOTH success and rejection. A
      // rejected entry that survived in the Map would wedge every future
      // create on this repo until Orca restarted (the F2 bug §3.3 pins).
      this.fetchInflight.delete(key)
    })

    this.fetchInflight.set(key, promise)
    return promise
  }

  async getOrStartRemoteTrackingBaseRefresh(
    repoPath: string,
    base: RemoteTrackingBase,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<RemoteFetchResult> {
    const remoteKey = await this.getCanonicalFetchKey(repoPath, base.remote, gitOptions)
    const key = await this.getCanonicalFetchKey(
      repoPath,
      `base:${base.remote}:${base.branch}`,
      gitOptions
    )
    if (this.getFreshFetchCompletedAt(key) !== null) {
      // Why: exact-base freshness is the safety boundary. A full remote fetch
      // can be narrowed by repo refspecs, so it must not prove this branch.
      return { ok: true }
    }

    const existing = this.fetchInflight.get(key)
    if (existing) {
      return existing
    }

    const promise = this.enqueueRemoteFetch(remoteKey, async () => {
      if (this.getFreshFetchCompletedAt(key) !== null) {
        return { ok: true }
      }
      // Why: this exact refresh gates worktree create; ordinary fetches still own maintenance.
      return gitExecFileAsync(
        [
          ...GIT_FETCH_SKIP_AUTO_MAINTENANCE_CONFIG_ARGS,
          'fetch',
          '--no-tags',
          base.remote,
          `+refs/heads/${base.branch}:${base.ref}`
        ],
        {
          cwd: repoPath,
          ...gitOptions,
          // Why: exact remote-base refresh is the network gate for worktree
          // creation, so honor repo SSH routing and bound custom wrappers.
          useConfiguredSshCommandForNetwork: true,
          timeout: REMOTE_FETCH_TIMEOUT_MS
        }
      )
        .then((): RemoteFetchResult => {
          this.rememberFreshFetchCompletedAt(key)
          return { ok: true }
        })
        .catch((err): RemoteFetchResult => {
          console.warn(
            `[refreshRemoteTrackingBase] ${base.base} refresh failed for ${repoPath}:`,
            err
          )
          return { ok: false, errorKind: 'git_error' }
        })
    }).finally(() => {
      this.fetchInflight.delete(key)
    })

    this.fetchInflight.set(key, promise)
    return promise
  }

  async fetchRemoteWithCache(
    repoPath: string,
    remote: string,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<void> {
    await this.getOrStartRemoteFetch(repoPath, remote, gitOptions)
  }

  async resolveRemoteTrackingBase(
    repoPath: string,
    baseBranch: string,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<RemoteTrackingBase | null> {
    let remotes: string[]
    try {
      const { stdout } = await gitExecFileAsync(['remote'], { cwd: repoPath, ...gitOptions })
      remotes = stdout
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
    } catch {
      return null
    }

    const remoteRefPrefix = 'refs/remotes/'
    const shortBaseBranch = baseBranch.startsWith(remoteRefPrefix)
      ? baseBranch.slice(remoteRefPrefix.length)
      : baseBranch
    const remote = remotes
      .filter((candidate) => shortBaseBranch.startsWith(`${candidate}/`))
      .sort((a, b) => b.length - a.length)[0]
    if (!remote) {
      return null
    }
    const branch = shortBaseBranch.slice(remote.length + 1)
    if (!branch) {
      return null
    }
    return {
      remote,
      branch,
      ref: `refs/remotes/${remote}/${branch}`,
      base: `${remote}/${branch}`
    }
  }

  async hasRemoteTrackingRef(
    repoPath: string,
    base: RemoteTrackingBase,
    gitOptions: { wslDistro?: string } = {}
  ): Promise<boolean> {
    try {
      await gitExecFileAsync(['rev-parse', '--verify', `${base.ref}^{commit}`], {
        cwd: repoPath,
        ...gitOptions
      })
      return true
    } catch {
      return false
    }
  }
}
