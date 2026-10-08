/**
 * ReviewBottomDock.tsx — FE-CV-TASK-059-05 / 060-03
 *
 * Collapsible bottom dock of the Review workspace. Hosts the panels from review-dock-registry
 * (structural findings, notes, and the quality lens's findings when its flag is on). A collapsed
 * dock mounts nothing, so closed panels issue no requests.
 *
 * @module components/review-map/shell/ReviewBottomDock
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import './review-dock-builtin-panels'
import { getReviewDockPanels } from './review-dock-registry'
import type { ReviewDockPanelProps } from './review-dock-registry'
import { subscribeReviewDockFocus } from './review-dock-focus'

const STORAGE_KEY = 'orca.review.dock'

type DockPrefs = { open: boolean; panelId: string | null }

function readPrefs(): DockPrefs {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    const parsed = raw ? (JSON.parse(raw) as Partial<DockPrefs>) : {}
    return {
      open: parsed.open === true,
      panelId: typeof parsed.panelId === 'string' ? parsed.panelId : null
    }
  } catch {
    return { open: false, panelId: null }
  }
}

function writePrefs(prefs: DockPrefs): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs))
  } catch {
    // Preference only; the dock works without persistence.
  }
}

export function ReviewBottomDock({
  flags,
  panelProps
}: {
  flags: { quality: boolean }
  panelProps: ReviewDockPanelProps
}): React.JSX.Element | null {
  const [prefs, setPrefs] = useState<DockPrefs>(() => readPrefs())
  const panels = useMemo(() => getReviewDockPanels(flags), [flags])
  const update = useCallback((patch: Partial<DockPrefs>) => {
    setPrefs((prev) => {
      const next = { ...prev, ...patch }
      writePrefs(next)
      return next
    })
  }, [])
  const worktreeId = panelProps.worktreeId
  useEffect(
    () =>
      subscribeReviewDockFocus((id, panelId) => {
        if (id === worktreeId) {
          update({ open: true, panelId })
        }
      }),
    [worktreeId, update]
  )

  if (panels.length === 0) {
    return null
  }
  const active = panels.find((p) => p.id === prefs.panelId) ?? panels[0]
  const toggleLabel = prefs.open
    ? translate('auto.components.reviewMap.shell.dock.collapse', 'Collapse panel')
    : translate('auto.components.reviewMap.shell.dock.expand', 'Expand panel')

  return (
    <section
      data-testid="review-dock"
      data-open={prefs.open}
      aria-label={translate('auto.components.reviewMap.shell.dock.label', 'Review panels')}
      className="flex shrink-0 flex-col border-t border-border"
    >
      <div role="group" className="flex items-center gap-1 px-2 py-1">
        {panels.map((p) => (
          <Button
            key={p.id}
            type="button"
            size="xs"
            variant={prefs.open && p.id === active.id ? 'secondary' : 'ghost'}
            aria-pressed={prefs.open && p.id === active.id}
            data-dock-panel={p.id}
            onClick={() => update({ open: true, panelId: p.id })}
          >
            {translate(p.labelKey, p.labelFallback)}
          </Button>
        ))}
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          className="ml-auto"
          aria-label={toggleLabel}
          aria-expanded={prefs.open}
          onClick={() => update({ open: !prefs.open })}
        >
          {prefs.open ? <ChevronDown aria-hidden /> : <ChevronUp aria-hidden />}
        </Button>
      </div>
      {prefs.open ? (
        <div className="flex h-56 min-h-0 flex-col overflow-y-auto">
          {active.render(panelProps)}
        </div>
      ) : null}
    </section>
  )
}
