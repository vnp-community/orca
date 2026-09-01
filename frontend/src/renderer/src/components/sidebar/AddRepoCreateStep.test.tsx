import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { Dialog } from '@/components/ui/dialog'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CreateStep } from './AddRepoCreateStep'
import type { GitAvailability } from './create-project-defaults'

function renderCreateStep({
  createName = '',
  gitAvailability = 'available',
  createParent = '/Users/alice/orca/projects',
  parentDefaultPending = false,
  manualParentEntry = false,
  devServerId
}: {
  createName?: string
  gitAvailability?: GitAvailability
  createParent?: string
  parentDefaultPending?: boolean
  manualParentEntry?: boolean
  devServerId?: string | null
} = {}): string {
  return renderToStaticMarkup(
    <TooltipProvider>
      <Dialog open>
        <CreateStep
          createName={createName}
          createParent={createParent}
          createError={null}
          isCreating={false}
          defaultParent="/Users/alice/orca/projects"
          gitAvailability={gitAvailability}
          runtimeParentStatus="idle"
          parentDefaultPending={parentDefaultPending}
          manualParentEntry={manualParentEntry}
          devServerId={devServerId}
          onNameChange={vi.fn()}
          onParentChange={vi.fn()}
          onPickParent={vi.fn()}
          onCreate={vi.fn()}
        />
      </Dialog>
    </TooltipProvider>
  )
}

describe('CreateStep', () => {
  it('renders the name-first create UI with advanced controls collapsed', () => {
    const html = renderCreateStep()

    expect(html).toContain('Create a new project')
    expect(html).toContain('Name')
    expect(html).toContain('Git repository in ~/orca/projects')
    // The summary card itself is the collapsed disclosure for the uncommon settings.
    expect(html).toContain('aria-expanded="false"')
    expect(html).not.toContain('Project kind')
    expect(html).not.toContain('Location</span>')
    expect(html).not.toContain('aria-label="Browse host filesystem"')
  })

  it('shows the Git-required explanation in the collapsed summary', () => {
    const html = renderCreateStep({
      createName: 'demo-project',
      gitAvailability: 'unavailable'
    })

    expect(html).toContain('Git repository in ~/orca/projects')
    expect(html).toContain('Git is required to create a project.')
    expect(html).toContain('disabled=""')
  })

  it('disables create while an auto-filled parent belongs to a previous target', () => {
    const html = renderCreateStep({
      createName: 'demo-project',
      parentDefaultPending: true
    })

    expect(html).toContain('disabled=""')
  })

  // Live-bug regression: "Choose parent folder..." never worked on a
  // dev-server-agent-only session (isRemoteHost/the Browse button's
  // disabled condition only ever checked runtimeEnvironmentId/sshTargetId).
  it('enables the Browse host filesystem button for a devServerId-only session', () => {
    const html = renderCreateStep({
      manualParentEntry: true,
      devServerId: 'ds-1'
    })

    expect(html).toContain('aria-label="Browse host filesystem"')
    // The button element must not carry a disabled attribute — a coarse but
    // reliable check given renderToStaticMarkup omits disabled="" entirely
    // when the prop is false.
    const browseButtonMatch = html.match(/<button[^>]*aria-label="Browse host filesystem"[^>]*>/)
    expect(browseButtonMatch).not.toBeNull()
    // A real disabled boolean prop renders as the literal disabled="" HTML
    // attribute — distinct from the "disabled:" Tailwind variant classes
    // this button's className always carries regardless of state.
    expect(browseButtonMatch?.[0]).not.toContain('disabled=""')
  })
})
