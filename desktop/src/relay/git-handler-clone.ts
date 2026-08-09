// ─── GitCloneHandler ────────────────────────────────────────────────────────────
// Relay handler for cloning a git repository with progress streaming.
// Registered in relay.ts alongside other handlers.

import { spawn } from 'node:child_process'
import type { RelayDispatcher, RequestContext } from './dispatcher'

export class GitCloneHandler {
  constructor(private dispatcher: RelayDispatcher) {
    this.dispatcher.onRequest('git.clone', (p, ctx) =>
      this.cloneRepo(p as { url: string; targetPath: string }, ctx)
    )
  }

  async cloneRepo(
    params: { url: string; targetPath: string },
    ctx: RequestContext
  ): Promise<{ path: string }> {
    return new Promise((resolve, reject) => {
      // We use node:child_process spawn with --progress to force progress output to stderr
      const child = spawn('git', ['clone', '--progress', params.url, params.targetPath], {
        stdio: ['ignore', 'pipe', 'pipe'],
        env: process.env
      })

      let errorOutput = ''

      child.stdout.on('data', (chunk: Buffer) => {
        // Not typically used by git clone, but we stream it just in case
        this.dispatcher.notify('git.clone.output', {
          data: chunk.toString('utf-8'),
          clientId: ctx.clientId
        })
      })

      child.stderr.on('data', (chunk: Buffer) => {
        const text = chunk.toString('utf-8')
        errorOutput += text
        // git clone --progress writes progress to stderr
        this.dispatcher.notify('git.clone.output', {
          data: text,
          clientId: ctx.clientId
        })
      })

      child.on('error', (err) => {
        reject(new Error(`Failed to start git clone: ${err.message}`))
      })

      child.on('exit', (code) => {
        if (code === 0) {
          resolve({ path: params.targetPath })
        } else {
          reject(new Error(`Git clone failed (exit code ${code}): ${errorOutput}`))
        }
      })
    })
  }
}
