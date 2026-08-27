/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
pre-existing repo-hooks/setup-script method block, already covered by
orca-runtime.ts's own grandfathered max-lines disable before this move.
Only marginally over budget (302 vs 300) after the move — a small
follow-on cleanup could bring it under without further splitting.
Registered in config/max-lines-baseline.txt per AGENTS.md — NEEDS PR
REVIEW. */
// frontend/src/main/runtime/orca-runtime-repo-hooks.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-043): repo hooks / setup-script
// inspection / issue-command commands extracted from OrcaRuntimeService via
// the composition pattern. Textually adjacent to the GitHub/GitLab
// issue-tracking domain (TASK-BIGFILE-042) but a conceptually separate
// concern (local orca.yaml / .orca/issue-command inspection, not
// GitHub/GitLab API calls) — deliberately kept as its own move.
import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import type { Repo } from '../../shared/types'
import type { IFilesystemProvider } from '../providers/types'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import { isFolderRepo } from '../../shared/repo-kind'
import { inspectSetupScriptImportCandidates } from '../../shared/setup-script-imports'
import { getRemoteFilesystemProvider } from '../providers/ssh-filesystem-dispatch'
import { isENOENT } from '../ipc/filesystem-auth'
import { joinWorktreeRelativePath } from './runtime-relative-paths'
import {
  getDefaultTabCommandTrustContent,
  getEffectiveHooks,
  getEffectiveSetupRunPolicy,
  hasHooksFile,
  hasUnrecognizedOrcaYamlKeys,
  loadHooks,
  parseOrcaYaml,
  readIssueCommand,
  writeIssueCommand
} from '../hooks'

export type RuntimeRepoHooksCommandHost = {
  resolveRepoSelector(selector: string): Promise<Repo>
}

export class RuntimeRepoHooksCommands {
  constructor(private readonly host: RuntimeRepoHooksCommandHost) {}

  private getSetupHookTrustPayload(
    repo: Repo,
    scriptContentValue: string | undefined
  ): { contentHash: string; scriptContent: string } | undefined {
    const scriptContent = scriptContentValue?.trim()
    if (!scriptContent || repo.hookSettings?.commandSourcePolicy === 'local-only') {
      return undefined
    }
    return {
      contentHash: createHash('sha256').update(scriptContent).digest('hex'),
      scriptContent
    }
  }

  private getSharedSetupHookTrustPayload(
    repo: Repo,
    sharedSetupScript: string | undefined
  ): { contentHash: string; scriptContent: string } | undefined {
    if (repo.hookSettings?.commandSourcePolicy === 'local-only') {
      return undefined
    }
    return this.getSetupHookTrustPayload(repo, sharedSetupScript)
  }

  async getRepoHooks(repoSelector: string) {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (providerConnectionId) {
      const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
      if (!fsProvider) {
        return {
          hasHooksFile: false,
          hooks: null,
          setupRunPolicy: getEffectiveSetupRunPolicy(repo),
          source: null
        }
      }
      try {
        const result = await fsProvider.readFile(joinWorktreeRelativePath(repo.path, 'orca.yaml'))
        const hooks = result.isBinary ? null : parseOrcaYaml(result.content)
        return {
          hasHooksFile: Boolean(hooks),
          hooks,
          setupRunPolicy: getEffectiveSetupRunPolicy(repo),
          source: hooks ? 'orca.yaml' : null,
          setupTrust: this.getSharedSetupHookTrustPayload(
            repo,
            getDefaultTabCommandTrustContent(hooks)
          )
        }
      } catch {
        return {
          hasHooksFile: false,
          hooks: null,
          setupRunPolicy: getEffectiveSetupRunPolicy(repo),
          source: null
        }
      }
    }
    const hasFile = hasHooksFile(repo.path)
    const hooks = getEffectiveHooks(repo)
    const sharedHooks = hasFile ? loadHooks(repo.path) : null
    const setupRunPolicy = getEffectiveSetupRunPolicy(repo)
    return {
      hasHooksFile: hasFile,
      hooks,
      setupRunPolicy,
      source: hasFile ? 'orca.yaml' : hooks ? 'legacy' : null,
      setupTrust: this.getSharedSetupHookTrustPayload(
        repo,
        getDefaultTabCommandTrustContent(sharedHooks)
      )
    }
  }

  async checkRepoHooks(repoSelector: string) {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return { hasHooks: false, hooks: null, mayNeedUpdate: false }
    }

    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (providerConnectionId) {
      const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
      if (!fsProvider) {
        return { hasHooks: false, hooks: null, mayNeedUpdate: false }
      }
      try {
        const result = await fsProvider.readFile(joinWorktreeRelativePath(repo.path, 'orca.yaml'))
        if (result.isBinary) {
          return { hasHooks: false, hooks: null, mayNeedUpdate: false }
        }
        return { hasHooks: true, hooks: parseOrcaYaml(result.content), mayNeedUpdate: false }
      } catch {
        return { hasHooks: false, hooks: null, mayNeedUpdate: false }
      }
    }

    const has = hasHooksFile(repo.path)
    const hooks = has ? loadHooks(repo.path) : null
    return {
      hasHooks: has,
      hooks,
      mayNeedUpdate: has && !hooks && hasUnrecognizedOrcaYamlKeys(repo.path)
    }
  }

  async inspectRepoSetupScriptImports(repoSelector: string) {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return []
    }

    return inspectSetupScriptImportCandidates(async (relativePath) => {
      const filePath = joinWorktreeRelativePath(repo.path, relativePath)
      const providerConnectionId = getRepoProviderConnectionKey(repo)
      if (providerConnectionId) {
        const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
        if (!fsProvider) {
          return null
        }
        try {
          const result = await fsProvider.readFile(filePath)
          return result.isBinary ? null : result.content
        } catch {
          return null
        }
      }

      try {
        return await readFile(filePath, 'utf-8')
      } catch (error) {
        if (!isENOENT(error)) {
          console.warn('[runtime] Failed to inspect setup script import candidate:', error)
        }
        return null
      }
    })
  }

  async readRepoIssueCommand(repoSelector: string) {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return {
        localContent: null,
        sharedContent: null,
        effectiveContent: null,
        localFilePath: '',
        source: 'none' as const
      }
    }

    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (providerConnectionId) {
      const issueCommandPath = joinWorktreeRelativePath(repo.path, '.orca/issue-command')
      const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
      if (!fsProvider) {
        return {
          localContent: null,
          sharedContent: null,
          effectiveContent: null,
          localFilePath: issueCommandPath,
          source: 'none' as const
        }
      }
      const localContent = await this.readRemoteIssueCommandOverride(fsProvider, issueCommandPath)
      const sharedContent = await this.readRemoteSharedIssueCommand(fsProvider, repo.path)
      const effectiveContent = localContent ?? sharedContent
      return {
        localContent,
        sharedContent,
        effectiveContent,
        localFilePath: issueCommandPath,
        source: localContent
          ? ('local' as const)
          : sharedContent
            ? ('shared' as const)
            : ('none' as const)
      }
    }

    return readIssueCommand(repo.path)
  }

  private async readRemoteIssueCommandOverride(
    fsProvider: IFilesystemProvider,
    issueCommandPath: string
  ): Promise<string | null> {
    try {
      const result = await fsProvider.readFile(issueCommandPath)
      if (result.isBinary) {
        return null
      }
      return result.content.trim() || null
    } catch {
      return null
    }
  }

  private async readRemoteSharedIssueCommand(
    fsProvider: IFilesystemProvider,
    repoPath: string
  ): Promise<string | null> {
    try {
      const result = await fsProvider.readFile(joinWorktreeRelativePath(repoPath, 'orca.yaml'))
      if (result.isBinary) {
        return null
      }
      return parseOrcaYaml(result.content)?.issueCommand?.trim() || null
    } catch {
      return null
    }
  }

  async writeRepoIssueCommand(repoSelector: string, content: string): Promise<{ ok: true }> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      return { ok: true }
    }

    const providerConnectionId = getRepoProviderConnectionKey(repo)
    if (providerConnectionId) {
      const issueCommandPath = joinWorktreeRelativePath(repo.path, '.orca/issue-command')
      const fsProvider = getRemoteFilesystemProvider(providerConnectionId)
      if (!fsProvider) {
        return { ok: true }
      }
      const trimmed = content.trim()
      if (!trimmed) {
        await fsProvider.deletePath(issueCommandPath, false).catch((error: unknown) => {
          if (!isENOENT(error)) {
            throw error
          }
        })
        return { ok: true }
      }
      await fsProvider.createDir(joinWorktreeRelativePath(repo.path, '.orca'))
      await this.ensureRemoteOrcaDirIgnored(fsProvider, repo.path)
      await fsProvider.writeFile(issueCommandPath, `${trimmed}\n`)
      return { ok: true }
    }

    writeIssueCommand(repo.path, content)
    return { ok: true }
  }

  private async ensureRemoteOrcaDirIgnored(
    fsProvider: IFilesystemProvider,
    repoPath: string,
    options: { required?: boolean } = {}
  ): Promise<void> {
    const gitignorePath = joinWorktreeRelativePath(repoPath, '.gitignore')
    let result: Awaited<ReturnType<IFilesystemProvider['readFile']>>
    try {
      result = await fsProvider.readFile(gitignorePath)
    } catch (error) {
      if (!isENOENT(error)) {
        if (options.required) {
          throw error
        }
        console.warn('[runtime] Could not inspect remote .gitignore for .orca', error)
        return
      }
      try {
        await fsProvider.writeFile(gitignorePath, '.orca\n')
      } catch (writeError) {
        if (options.required) {
          throw writeError
        }
        console.warn('[runtime] Could not update remote .gitignore to exclude .orca', writeError)
      }
      return
    }
    if (result.isBinary) {
      if (options.required) {
        throw new Error('Remote .gitignore is binary; cannot verify .orca is ignored')
      }
      return
    }
    if (/^\.orca\/?$/m.test(result.content)) {
      return
    }
    const separator = result.content.endsWith('\n') ? '' : '\n'
    try {
      await fsProvider.writeFile(gitignorePath, `${result.content}${separator}.orca\n`)
    } catch (writeError) {
      if (options.required) {
        throw writeError
      }
      console.warn('[runtime] Could not update remote .gitignore to exclude .orca', writeError)
    }
  }
}
