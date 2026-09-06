// @vitest-environment happy-dom

import React from 'react'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Repo } from '../../../../shared/types'
import { getLocalCommandSourcePolicyNotice, RepositoryHooksSection } from './RepositoryHooksSection'

vi.mock('@/store', () => ({
  useAppStore: (selector: (state: unknown) => unknown) =>
    selector({
      settings: {},
      settingsSearchQuery: ''
    })
}))

vi.mock('@/runtime/runtime-hooks-client', () => ({
  readRuntimeIssueCommand: vi.fn().mockResolvedValue({ command: '', exists: false }),
  writeRuntimeIssueCommand: vi.fn().mockResolvedValue(undefined)
}))

const repo: Repo = {
  id: 'repo-1',
  kind: 'git',
  path: '/workspace/repo',
  displayName: 'Repo',
  badgeColor: 'blue',
  addedAt: 1,
  gitUsername: ''
}

function renderRepositoryHooksSection(args: {
  onUpdateHookSettings: (settings: NonNullable<Repo['hookSettings']>) => void
}): { container: HTMLDivElement; root: Root } {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => {
    root.render(
      React.createElement(RepositoryHooksSection, {
        repo,
        yamlHooks: null,
        hasHooksFile: false,
        hooksInspectionReady: true,
        mayNeedUpdate: false,
        copiedTemplate: false,
        forceVisible: true,
        onCopyTemplate: () => {},
        onUpdateHookSettings: args.onUpdateHookSettings
      })
    )
  })
  return { container, root }
}

let rendered: { container: HTMLDivElement; root: Root } | null = null

afterEach(() => {
  act(() => rendered?.root.unmount())
  rendered?.container.remove()
  rendered = null
})

describe('getLocalCommandSourcePolicyNotice', () => {
  it('does not show a notice when no local scripts are saved', () => {
    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: true,
        currentPolicy: 'shared-only',
        setupScript: '',
        archiveScript: '',
        hasSharedScript: false
      })
    ).toBeNull()
  })

  it('does not show a notice when command source already includes local scripts', () => {
    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: true,
        currentPolicy: 'local-only',
        setupScript: 'pnpm install',
        archiveScript: '',
        hasSharedScript: true
      })
    ).toBeNull()

    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: true,
        currentPolicy: 'run-both',
        setupScript: '',
        archiveScript: 'echo archive',
        hasSharedScript: true
      })
    ).toBeNull()
  })

  it('waits for hook inspection before recommending a command source', () => {
    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: false,
        currentPolicy: 'shared-only',
        setupScript: 'pnpm install',
        archiveScript: '',
        hasSharedScript: false
      })
    ).toEqual({ kind: 'checking' })
  })

  it('recommends local commands when local scripts are saved and no shared script exists', () => {
    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: true,
        currentPolicy: 'shared-only',
        setupScript: 'pnpm install',
        archiveScript: '',
        hasSharedScript: false
      })
    ).toEqual({ kind: 'action', policy: 'local-only', label: 'Use local commands' })
  })

  it('recommends run-both when local and shared scripts both exist', () => {
    expect(
      getLocalCommandSourcePolicyNotice({
        hooksInspectionReady: true,
        currentPolicy: 'shared-only',
        setupScript: '',
        archiveScript: 'echo archive',
        hasSharedScript: true
      })
    ).toEqual({ kind: 'action', policy: 'run-both', label: 'Run both' })
  })
})

describe('RepositoryHooksSection Setup Script draft survives unrelated re-renders', () => {
  it('does not wipe just-typed text when onUpdateHookSettings gets a new identity mid-debounce', () => {
    // Regression for a live bug: an unmemoized `onUpdateHookSettings` prop
    // (a new function every parent render, e.g. from live terminal/agent
    // status updates unrelated to this section) re-ran the resync effect
    // while `repo.hookSettings` still held the pre-edit value — wiping text
    // the user had just typed, ~700ms after they stopped typing.
    vi.useFakeTimers()
    try {
      const updates: NonNullable<Repo['hookSettings']>[] = []
      const onUpdateHookSettingsA = (settings: NonNullable<Repo['hookSettings']>): void => {
        updates.push(settings)
      }
      rendered = renderRepositoryHooksSection({ onUpdateHookSettings: onUpdateHookSettingsA })

      const textarea = rendered.container.querySelector<HTMLTextAreaElement>(
        'textarea[aria-label="Setup Script"]'
      )
      expect(textarea).toBeTruthy()

      const nativeSetter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement.prototype,
        'value'
      )!.set!
      act(() => {
        nativeSetter.call(textarea, 'pnpm install')
        textarea!.dispatchEvent(new Event('input', { bubbles: true }))
      })
      expect(textarea!.value).toBe('pnpm install')

      // Advance past the 700ms autosave debounce — this clears the dirty
      // flag and calls onUpdateHookSettings, but repo.hookSettings (the
      // store) has NOT been updated yet in this test, matching the real
      // async gap between "saved" and "store reflects the save".
      act(() => {
        vi.advanceTimersByTime(800)
      })
      expect(updates.length).toBeGreaterThan(0)

      // Simulate the exact trigger: an unrelated parent re-render passes a
      // brand-new onUpdateHookSettings function, with the SAME repo object
      // (hookSettings unchanged/stale) — this used to re-run the resync
      // effect and clobber the draft back to the pre-edit (empty) value.
      const onUpdateHookSettingsB = (settings: NonNullable<Repo['hookSettings']>): void => {
        updates.push(settings)
      }
      act(() => {
        rendered!.root.render(
          React.createElement(RepositoryHooksSection, {
            repo,
            yamlHooks: null,
            hasHooksFile: false,
            hooksInspectionReady: true,
            mayNeedUpdate: false,
            copiedTemplate: false,
            forceVisible: true,
            onCopyTemplate: () => {},
            onUpdateHookSettings: onUpdateHookSettingsB
          })
        )
      })

      const textareaAfterRerender = rendered.container.querySelector<HTMLTextAreaElement>(
        'textarea[aria-label="Setup Script"]'
      )
      expect(textareaAfterRerender!.value).toBe('pnpm install')
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('RepositoryHooksSection setup startup policy', () => {
  it('persists wait-for-setup when the repository toggle is checked', () => {
    const updates: NonNullable<Repo['hookSettings']>[] = []
    rendered = renderRepositoryHooksSection({
      onUpdateHookSettings: (settings) => updates.push(settings)
    })

    const waitSwitch = rendered.container.querySelector<HTMLElement>(
      '[role="switch"][aria-label="Wait for setup to complete before starting agent"]'
    )
    expect(waitSwitch).toBeTruthy()

    act(() => waitSwitch?.click())

    expect(updates.at(-1)).toMatchObject({
      setupAgentStartupPolicy: 'wait-for-setup',
      setupRunPolicy: 'run-by-default',
      scripts: { setup: '', archive: '' }
    })
  })
})
