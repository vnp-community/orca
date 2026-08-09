// Shared types cho Workspace + File Explorer (TDD-FE-12, 17)

export type OrcaProject = {
  id:            string
  name:          string
  description?:  string
  repoPath:      string
  defaultBranch: string
  devServerId:   string
  visibility:    'private' | 'team' | 'public'
  createdAt:     number
  updatedAt:     number
}

export type ProjectMember = {
  userId: string
  email:  string
  name:   string
  role:   'owner' | 'member' | 'viewer'
}

export type FileNode = {
  name:       string
  path:       string          // relative to project root
  type:       'file' | 'directory'
  size?:      number          // bytes (files only)
  children?:  FileNode[]      // lazy loaded
  isLoading?: boolean
}

export type GitStatus = {
  branch:         string
  aheadBy:        number
  behindBy:       number
  hasUncommitted: boolean
  staged:         number
  unstaged:       number
}
