import type { StateCreator } from 'zustand'
import type { AppState } from '../types'

export type GitFileStatus = 'M' | 'A' | 'D' | 'R' | 'U'

export type GitFileChange = {
  path: string
  status: GitFileStatus
  staged: boolean
}

export type GitCommit = {
  hash: string
  shortHash: string
  message: string
  author: string
  date: number
}

export type GitBranch = {
  name: string
  isRemote: boolean
  isCurrent: boolean
  upstream?: string
  aheadBy: number
  behindBy: number
}

export type GitPanelSliceState = {
  stagedFiles: GitFileChange[]
  unstagedFiles: GitFileChange[]
  gitHistory: GitCommit[]
  branches: GitBranch[]
  selectedDiffFile: string | null
  diffContent: string | null
  pushLines: string[]
  isPushing: boolean
  isCommitting: boolean
}

export type GitPanelSliceActions = {
  setStagedFiles(files: GitFileChange[]): void
  setUnstagedFiles(files: GitFileChange[]): void
  setGitHistory(commits: GitCommit[]): void
  setBranches(branches: GitBranch[]): void
  setSelectedDiffFile(path: string | null): void
  setDiffContent(diff: string | null): void
  appendPushLine(line: string): void
  clearPushLines(): void
  setIsPushing(v: boolean): void
  setIsCommitting(v: boolean): void
}

export type GitPanelSlice = GitPanelSliceState & GitPanelSliceActions

// Why every action returns a partial object instead of mutating `s` and
// returning nothing: this store has no immer middleware, so plain zustand's
// `set` treats a non-object return value (i.e. `undefined`, from a bare
// `set(s => { s.stagedFiles = files })`) as a full-state REPLACE — wiping
// the entire AppState to `undefined`. Same bug class fixed in task.ts's own
// doc comment (BUG-FE-TASKGRAPH-SETTINGS) — this slice had it too, just
// never live-triggered yet since GitPanel's fetches haven't succeeded on
// this deployment (worktree-scoped git.status has no worktree to target).
export const createGitPanelSlice: StateCreator<AppState, [], [], GitPanelSlice> = (set) => ({
  stagedFiles: [],
  unstagedFiles: [],
  gitHistory: [],
  branches: [],
  selectedDiffFile: null,
  diffContent: null,
  pushLines: [],
  isPushing: false,
  isCommitting: false,

  setStagedFiles: (files) => set(() => ({ stagedFiles: files })),
  setUnstagedFiles: (files) => set(() => ({ unstagedFiles: files })),
  setGitHistory: (c) => set(() => ({ gitHistory: c })),
  setBranches: (b) => set(() => ({ branches: b })),
  setSelectedDiffFile: (path) => set(() => ({ selectedDiffFile: path })),
  setDiffContent: (diff) => set(() => ({ diffContent: diff })),
  appendPushLine: (line) => set((s) => ({ pushLines: [...s.pushLines, line] })),
  clearPushLines: () => set(() => ({ pushLines: [] })),
  setIsPushing: (v) => set(() => ({ isPushing: v })),
  setIsCommitting: (v) => set(() => ({ isCommitting: v }))
})
