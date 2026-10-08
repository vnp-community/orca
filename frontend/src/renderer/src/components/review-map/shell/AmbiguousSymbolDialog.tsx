import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList
} from '@/components/ui/command'
import { translate } from '@/i18n/i18n'

export type AmbiguousSymbolCandidate = {
  key?: string
  uid: string
  name: string
  kind: string
  filePath: string
  line: number
  score?: number
  impactedCount?: number
  risk?: string
}

/** Re-query target: stable key when the backend gave one, else name + file. */
export type AmbiguousSymbolChoice = { key: string } | { name: string; file: string }

type Props = {
  candidates: readonly AmbiguousSymbolCandidate[]
  onChoose: (choice: AmbiguousSymbolChoice) => void
  onCancel: () => void
}

export function toSymbolChoice(c: AmbiguousSymbolCandidate): AmbiguousSymbolChoice {
  return c.key ? { key: c.key } : { name: c.name, file: c.filePath }
}

export function AmbiguousSymbolDialog({
  candidates,
  onChoose,
  onCancel
}: Props): React.JSX.Element {
  return (
    <Dialog open onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {translate('auto.components.reviewMap.shell.ambiguous.title', 'Which symbol?')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.components.reviewMap.shell.ambiguous.body',
              'More than one symbol matches. Pick the one you mean.'
            )}
          </DialogDescription>
        </DialogHeader>
        <Command>
          {/* autoFocus: typing narrows the list immediately; Enter picks the highlighted row. */}
          <CommandInput
            autoFocus
            placeholder={translate(
              'auto.components.reviewMap.shell.ambiguous.search',
              'Search matches'
            )}
          />
          <CommandList>
            <CommandEmpty>
              {translate('auto.components.reviewMap.shell.ambiguous.empty', 'No match')}
            </CommandEmpty>
            {candidates.slice(0, 10).map((c) => (
              <CommandItem
                key={c.uid}
                value={`${c.name} ${c.filePath} ${c.kind}`}
                onSelect={() => onChoose(toSymbolChoice(c))}
              >
                <div className="min-w-0">
                  <div className="truncate text-sm">
                    {c.name} <span className="text-xs text-muted-foreground">{c.kind}</span>
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {c.filePath}:{c.line}
                    {typeof c.impactedCount === 'number' ? ` · ${c.impactedCount}` : ''}
                    {c.risk ? ` · ${c.risk}` : ''}
                  </div>
                </div>
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  )
}
