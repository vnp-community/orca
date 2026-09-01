// ProjectSettings.tsx — Project settings dialog with General/Members/Repos tabs (TDD-FE-12, TASK-FE-004)
import { useEffect, useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { MemberManager } from './MemberManager'
import { RepoMemberManager } from './RepoMemberManager'
import { LinkedProjectsManager } from './LinkedProjectsManager'
import { ProjectDevServerSection } from './ProjectDevServerSection'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import type { ProjectMember } from '../../types/workspace-types'

type ProjectSettingsProps = {
  projectId: string
  open: boolean
  onClose: () => void
}

type RepoListItem = { id: string; displayName: string; url: string }

export function ProjectSettings({ projectId, open, onClose }: ProjectSettingsProps) {
  // Why WorkspaceContext, not `useAppStore(s => s.projects)`: that field is
  // the legacy RepoSlice's own `projects` (multi-host repo grouping, an
  // unrelated concept) — casting it to OrcaProject[] never actually
  // resolved this project's real name. WorkspaceContext.project is the
  // OrcaProject ProjectSwitcher/switchProject actually fetched.
  const { project } = useWorkspace()
  const [activeTab, setActiveTab] = useState('general')
  const [repos, setRepos] = useState<RepoListItem[]>([])
  const [selectedRepoId, setSelectedRepoId] = useState<string | null>(null)
  const [currentUserRole, setCurrentUserRole] = useState<ProjectMember['role'] | null>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<{ repos: RepoListItem[] }>(target, 'repo.list', { projectId })
      .then((result) => setRepos(result.repos ?? []))
      .catch(() => setRepos([]))
  }, [open, projectId])

  // BUG-FE-PW-002 — the Linked Projects tab needs to know the viewer's own
  // role to decide whether to render the Unlink button. There is no RPC that
  // returns "my own membership row" directly (project.getMember is an
  // internal ProjectService.ts method, not a registered RPC — confirmed via
  // grep, see TASK-FE-PW-002-C), so reuse project.getMembers (already used by
  // MemberManager below) and filter by the signed-in user's id.
  useEffect(() => {
    if (!open) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const myUserId = useAppStore.getState().currentUser?.id
    if (!myUserId) {
      setCurrentUserRole(null)
      return
    }
    callRuntimeRpc<ProjectMember[]>(target, 'project.getMembers', { projectId })
      .then((members) => {
        const me = members.find((m) => m.userId === myUserId)
        setCurrentUserRole(me?.role ?? null)
      })
      .catch(() => setCurrentUserRole(null))
  }, [open, projectId])

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl" data-testid="project-settings-dialog">
        <DialogHeader>
          <DialogTitle>Project Settings — {project?.name ?? projectId}</DialogTitle>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="general" data-testid="tab-general">
              General
            </TabsTrigger>
            <TabsTrigger value="members" data-testid="tab-members">
              Members
            </TabsTrigger>
            <TabsTrigger value="repos" data-testid="tab-repos">
              Repos
            </TabsTrigger>
            <TabsTrigger value="linked" data-testid="tab-linked">
              Linked Projects
            </TabsTrigger>
          </TabsList>

          <TabsContent value="general" className="py-4">
            <div className="space-y-6">
              <p className="text-sm text-muted-foreground">
                General project settings (name, description, repository bindings).
              </p>
              {/* TODO: Add name/description form fields in future tasks */}
              <ProjectDevServerSection projectId={projectId} />
            </div>
          </TabsContent>

          <TabsContent value="members" className="py-2">
            <MemberManager projectId={projectId} />
          </TabsContent>

          <TabsContent value="repos" className="py-2">
            <div className="space-y-3">
              <p className="text-xs text-muted-foreground">
                Pick a repo to manage its functional-role grants (developer/lead/admin) — separate
                from project membership above.
              </p>
              <div className="flex flex-wrap gap-1.5" data-testid="repo-picker">
                {repos.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No repos in this project yet.</p>
                ) : (
                  repos.map((r) => (
                    <button
                      key={r.id}
                      type="button"
                      data-testid={`repo-picker-item-${r.id}`}
                      onClick={() => setSelectedRepoId(r.id)}
                      className={`rounded-md border px-2 py-1 text-xs ${
                        selectedRepoId === r.id ? 'border-primary bg-primary/10' : 'border-input'
                      }`}
                    >
                      {r.displayName || r.url}
                    </button>
                  ))
                )}
              </div>
              {selectedRepoId ? <RepoMemberManager repoId={selectedRepoId} /> : null}
            </div>
          </TabsContent>

          <TabsContent value="linked" className="py-2">
            <LinkedProjectsManager orcaProjectId={projectId} currentUserRole={currentUserRole} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
