// ─── FsDirectoryBrowserHandler ────────────────────────────────────────────────
// Relay handler for browsing remote directories.
// Registered in relay.ts alongside other relay handlers.
//
// Provides: fs.listDirectory

import { readdir, stat } from 'node:fs/promises'
import { join } from 'node:path'
import type { RelayDispatcher } from './dispatcher'

export type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  /** true if the directory contains a `.git` subfolder */
  isGitRepo: boolean
}

export class FsDirectoryBrowserHandler {
  constructor(private dispatcher: RelayDispatcher) {
    this.dispatcher.onRequest('fs.listDirectory', (p) =>
      this.listDirectory(p as { path: string; includeGitStatus?: boolean })
    )
  }

  private async listDirectory(params: {
    path: string
    includeGitStatus?: boolean
  }): Promise<{
    entries: DirectoryEntry[]
    platform: NodeJS.Platform
  }> {
    const { path: dirPath, includeGitStatus = false } = params

    let entries: DirectoryEntry[]
    try {
      const items = await readdir(dirPath, { withFileTypes: true })
      entries = await Promise.all(
        items
          .filter((item) => item.isDirectory())
          .map(async (item) => {
            const fullPath = join(dirPath, item.name)
            const isGitRepo = includeGitStatus ? await this.isGitRepo(fullPath) : false
            return {
              name: item.name,
              path: fullPath,
              isDirectory: true,
              isGitRepo
            }
          })
      )
    } catch (err) {
      throw new Error(
        `Cannot list directory ${dirPath}: ${(err as Error).message}`
      )
    }

    return { entries, platform: process.platform }
  }

  private async isGitRepo(dirPath: string): Promise<boolean> {
    try {
      await stat(join(dirPath, '.git'))
      return true
    } catch {
      return false
    }
  }
}
