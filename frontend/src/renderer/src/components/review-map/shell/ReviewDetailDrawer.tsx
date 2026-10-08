import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { translate } from '@/i18n/i18n'

type Props = {
  /** 'panel' = right column of the 3-column layout; 'sheet' = overlay on narrower tabs. */
  mode: 'panel' | 'sheet'
  onClose: () => void
  /** Element focused again when the drawer closes (Esc returns focus to the trigger). */
  returnFocusTo?: HTMLElement | null
  children: React.ReactNode
}

export function ReviewDetailDrawer({
  mode,
  onClose,
  returnFocusTo,
  children
}: Props): React.JSX.Element {
  const title = translate('auto.components.reviewMap.shell.drawer.title', 'Details')
  const returnRef = useRef(returnFocusTo ?? null)
  returnRef.current = returnFocusTo ?? returnRef.current
  useEffect(() => () => returnRef.current?.focus?.(), [])

  if (mode === 'sheet') {
    return (
      <Sheet open onOpenChange={(open) => !open && onClose()}>
        <SheetContent side="right" aria-describedby={undefined}>
          <SheetTitle className="px-4 pt-4">{title}</SheetTitle>
          <SheetDescription className="sr-only">{title}</SheetDescription>
          <div className="min-h-0 flex-1 overflow-auto p-4">{children}</div>
        </SheetContent>
      </Sheet>
    )
  }
  return (
    <aside aria-label={title} className="flex h-full min-h-0 flex-col border-l" data-drawer="panel">
      <div className="flex items-center justify-between px-3 py-2">
        <h2 className="text-sm font-medium">{title}</h2>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={onClose}
          aria-label={translate('auto.components.reviewMap.shell.drawer.close', 'Close details')}
        >
          <X aria-hidden />
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-3">{children}</div>
    </aside>
  )
}
