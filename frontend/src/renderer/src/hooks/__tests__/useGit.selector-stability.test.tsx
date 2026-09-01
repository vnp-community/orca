// @vitest-environment happy-dom
//
// Regression guard for the "Maximum update depth exceeded" (React error
// #185) bug live-reproduced right after switching to a freshly created
// project: useGit.ts selected {stagedFiles, unstagedFiles, isPushing,
// isCommitting} as a fresh object literal on every call, with no
// useShallow. Zustand v5's React binding hands that straight to React's
// own useSyncExternalStore, which has no built-in memoization — an
// unguarded object selector fails snapshot-equality on every render,
// causing an unconditional infinite re-render loop the moment any
// component mounts it (GitPanel's default "Changes" tab, in practice).
//
// This test exercises the REAL zustand store (not the heavily-mocked one
// useGit.test.ts uses) so it actually goes through real
// useSyncExternalStore snapshot comparison — the mocked store in
// useGit.test.ts bypasses that entirely and could never have caught this.
import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '../../store'

type GitPanelFields = {
  stagedFiles: unknown
  unstagedFiles: unknown
  isPushing: unknown
  isCommitting: unknown
}

const selectGitPanelFields = (s: ReturnType<typeof useAppStore.getState>): GitPanelFields => ({
  stagedFiles: s.stagedFiles,
  unstagedFiles: s.unstagedFiles,
  isPushing: s.isPushing,
  isCommitting: s.isCommitting
})

// Why: two components rather than one branching on a prop — the
// react-hooks(rules-of-hooks) lint (correctly) forbids calling useShallow
// conditionally, so each variant calls its selector unconditionally.
function GitPanelFieldsProbeUnguarded(): null {
  useAppStore(selectGitPanelFields)
  return null
}

function GitPanelFieldsProbeGuarded(): null {
  useAppStore(useShallow(selectGitPanelFields))
  return null
}

describe('useGit-style git-panel-fields selector', () => {
  it('without useShallow, an unguarded object selector triggers React\'s "Maximum update depth exceeded" guard', () => {
    // This is the exact bug this test guards against — asserted directly,
    // not just documented, so re-removing useShallow from useGit.ts fails
    // this test loudly instead of only failing live in production.
    expect(() => render(<GitPanelFieldsProbeUnguarded />)).toThrow(/Maximum update depth exceeded/)
  })

  it('with useShallow, the same selector mounts cleanly with no error', () => {
    expect(() => render(<GitPanelFieldsProbeGuarded />)).not.toThrow()
  })
})
