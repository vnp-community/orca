// ProjectSettings.tsx — Project settings dialog with General + Members tabs (TDD-FE-12, TASK-FE-004)
import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { MemberManager } from './MemberManager'
import { useAppStore } from '../../store'
import type { OrcaProject } from '../../types/workspace-types'

interface ProjectSettingsProps {
  projectId: string
  open:      boolean
  onClose:   () => void
}

export function ProjectSettings({ projectId, open, onClose }: ProjectSettingsProps) {
  const projects = useAppStore(s => s.projects as OrcaProject[])
  const project  = projects.find(p => p.id === projectId)
  const [activeTab, setActiveTab] = useState('general')

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl" data-testid="project-settings-dialog">
        <DialogHeader>
          <DialogTitle>
            Project Settings — {project?.name ?? projectId}
          </DialogTitle>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="general" data-testid="tab-general">General</TabsTrigger>
            <TabsTrigger value="members" data-testid="tab-members">Members</TabsTrigger>
          </TabsList>

          <TabsContent value="general" className="py-4">
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                General project settings (name, description, repository bindings).
              </p>
              {/* TODO: Add name/description form fields in future tasks */}
            </div>
          </TabsContent>

          <TabsContent value="members" className="py-2">
            <MemberManager projectId={projectId} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
