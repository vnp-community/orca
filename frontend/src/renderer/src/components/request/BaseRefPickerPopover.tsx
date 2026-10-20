import React, { useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { translate } from '@/i18n/i18n'
import { RepoBranchPicker } from '../git-ref-picker/RepoBranchPicker'
import { StartFromManualEntry } from './StartFromManualEntry'

const T = 'auto.components.request.StartFromField.'

type Props = {
  repoId: string
  /** The ref in effect; empty or undefined means the repository default. */
  value?: string
  /** The repository default as `repo.baseRefDefault` names it, pinned on top of the list; null when it cannot be named. */
  defaultName?: string | null
  /** The host could not be asked for branches: only typing a name is offered. */
  offline: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  /** A ref the user picked or typed; never empty. */
  onPick: (ref: string) => void
  onUseDefault: () => void
  /** The element that opens the popover (a button). */
  children: React.ReactNode
}

/**
 * Branch chooser shared by the create dialog, the Overview row and the failure card: pick from the repo's branches (or search
 * them on a gateway that cannot list), fall back to the repository default, or type a name when the host is out of reach.
 */
export function BaseRefPickerPopover({
  repoId,
  value,
  defaultName = null,
  offline,
  open,
  onOpenChange,
  onPick,
  onUseDefault,
  children
}: Props): React.JSX.Element {
  const [typing, setTyping] = useState(false)
  const contentRef = useRef<HTMLDivElement>(null)

  // Radix closes a popover on Escape in the capture phase, before a field can clear its text: keep it open while there is text.
  const keepOpenWhileTextIsTyped = (event: KeyboardEvent): void => {
    const active = document.activeElement
    if (
      active instanceof HTMLInputElement &&
      active.value !== '' &&
      contentRef.current?.contains(active)
    ) {
      event.preventDefault()
    }
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setTyping(false)
        }
        onOpenChange(next)
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent
        ref={contentRef}
        align="start"
        className="w-[min(22rem,90vw)] space-y-3 p-3"
        onEscapeKeyDown={keepOpenWhileTextIsTyped}
      >
        {offline ? (
          <p role="status" className="text-xs text-muted-foreground">
            {translate(
              `${T}offline`,
              'Cannot reach the host to list branches. Type the branch name; it is checked when the worktree is created.'
            )}
          </p>
        ) : (
          <RepoBranchPicker
            repoId={repoId}
            value={value || undefined}
            defaultName={defaultName}
            onSelect={onPick}
            onUseDefault={onUseDefault}
          />
        )}
        {typing || offline ? (
          <StartFromManualEntry onSubmit={onPick} />
        ) : (
          <Button
            type="button"
            variant="link"
            size="xs"
            className="h-auto px-0"
            onClick={() => setTyping(true)}
          >
            {translate(`${T}typeInstead`, 'Type a branch name instead')}
          </Button>
        )}
      </PopoverContent>
    </Popover>
  )
}
