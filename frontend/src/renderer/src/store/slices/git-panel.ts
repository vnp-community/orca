import type { StateCreator } from 'zustand'

export type GitFileStatus = 'M' | 'A' | 'D' | 'R' | 'U'

export type GitFileChange = {
  path:   string
  status: GitFileStatus
  staged: boolean
}

export type GitCommit = {
  hash:      string
  shortHash: string
  message:   string
  author:    string
  date:      number
}

export type GitBranch = {
  name:      string
  isRemote:  boolean
  isCurrent: boolean
  upstream?: string
  aheadBy:   number
  behindBy:  number
}

export type GitPanelSliceState = {
  stagedFiles:      GitFileChange[]
  unstagedFiles:    GitFileChange[]
  gitHistory:       GitCommit[]
  branches:         GitBranch[]
  selectedDiffFile: string | null
  diffContent:      string | null
  pushLines:        string[]
  isPushing:        boolean
  isCommitting:     boolean
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

export function createGitPanelSlice(
  set: StateCreator<GitPanelSlice>['arguments'][0]
): GitPanelSlice {
  return {
    stagedFiles:      [],
    unstagedFiles:    [],
    gitHistory:       [],
    branches:         [],
    selectedDiffFile: null,
    diffContent:      null,
    pushLines:        [],
    isPushing:        false,
    isCommitting:     false,

    setStagedFiles:      (files)  => set(s => { s.stagedFiles = files }),
    setUnstagedFiles:    (files)  => set(s => { s.unstagedFiles = files }),
    setGitHistory:       (c)      => set(s => { s.gitHistory = c }),
    setBranches:         (b)      => set(s => { s.branches = b }),
    setSelectedDiffFile: (path)   => set(s => { s.selectedDiffFile = path }),
    setDiffContent:      (diff)   => set(s => { s.diffContent = diff }),
    appendPushLine:      (line)   => set(s => { s.pushLines.push(line) }),
    clearPushLines:      ()       => set(s => { s.pushLines = [] }),
    setIsPushing:        (v)      => set(s => { s.isPushing = v }),
    setIsCommitting:     (v)      => set(s => { s.isCommitting = v }),
  }
}
