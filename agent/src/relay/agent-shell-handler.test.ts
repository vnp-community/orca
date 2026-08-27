import { describe, expect, it, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { mkdtempSync, writeFileSync, existsSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { handleFsCopyFile } from './agent-shell-handler'
import type { AgentConfig } from './agent-config'

function testConfig(workDir: string): AgentConfig {
  return { workDir } as AgentConfig
}

describe('handleFsCopyFile', () => {
  let tmpDir: string
  let srcDir: string
  let workDir: string

  beforeEach(() => {
    tmpDir = mkdtempSync(path.join(tmpdir(), 'agent-fs-copy-'))
    srcDir = path.join(tmpDir, 'src')
    workDir = path.join(tmpDir, 'work')
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('copies a file from any absolute source into the workDir', async () => {
    await fs.mkdir(srcDir, { recursive: true })
    await fs.mkdir(workDir, { recursive: true })
    const srcPath = path.join(srcDir, 'image.png')
    writeFileSync(srcPath, 'binary-content')
    const destPath = path.join(workDir, 'image.png')

    const response = (await handleFsCopyFile(
      1,
      { srcPath, destPath },
      testConfig(workDir)
    )) as { result?: { ok: boolean; path: string } }

    expect(response.result?.ok).toBe(true)
    expect(existsSync(destPath)).toBe(true)
    expect(readFileSync(destPath, 'utf8')).toBe('binary-content')
  })

  it('refuses to overwrite an existing destination', async () => {
    await fs.mkdir(srcDir, { recursive: true })
    await fs.mkdir(workDir, { recursive: true })
    const srcPath = path.join(srcDir, 'image.png')
    const destPath = path.join(workDir, 'image.png')
    writeFileSync(srcPath, 'new-content')
    writeFileSync(destPath, 'existing-content')

    const response = (await handleFsCopyFile(1, { srcPath, destPath }, testConfig(workDir))) as {
      error?: { message: string }
    }

    expect(response.error).toBeDefined()
    expect(readFileSync(destPath, 'utf8')).toBe('existing-content')
  })

  it('rejects a destination outside workDir', async () => {
    await fs.mkdir(srcDir, { recursive: true })
    await fs.mkdir(workDir, { recursive: true })
    const srcPath = path.join(srcDir, 'image.png')
    writeFileSync(srcPath, 'content')
    const destPath = path.join(tmpDir, 'outside', 'image.png')

    const response = (await handleFsCopyFile(1, { srcPath, destPath }, testConfig(workDir))) as {
      error?: { message: string }
    }

    expect(response.error?.message).toContain('outside project root')
  })

  it('rejects relative paths', async () => {
    const response = (await handleFsCopyFile(
      1,
      { srcPath: 'relative.png', destPath: 'also-relative.png' },
      testConfig(workDir)
    )) as { error?: { message: string } }

    expect(response.error?.message).toContain('absolute paths')
  })

  it('rejects missing params', async () => {
    const response = (await handleFsCopyFile(1, {}, testConfig(workDir))) as {
      error?: { message: string }
    }

    expect(response.error?.message).toContain('Missing required params')
  })
})
