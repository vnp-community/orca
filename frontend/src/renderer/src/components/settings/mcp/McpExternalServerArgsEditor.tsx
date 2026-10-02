import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { looksLikeSecretArg } from './mcp-external-server-validation'

/** One input per argument (no single textarea) so quoting can never change argument boundaries. */
export function McpExternalServerArgsEditor({
  args,
  onChange
}: {
  args: string[]
  onChange: (next: string[]) => void
}): React.JSX.Element {
  const move = (i: number, by: -1 | 1): void => {
    const next = [...args]
    ;[next[i], next[i + by]] = [next[i + by], next[i]]
    onChange(next)
  }
  return (
    <div className="space-y-2">
      {args.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {translate('auto.mcp.external.noArgs', 'No arguments.')}
        </p>
      ) : null}
      {args.map((a, i) => {
        const n = i + 1
        return (
          <div key={i} className="space-y-1">
            <div className="flex items-center gap-1">
              <Input
                value={a}
                onChange={(e) => onChange(args.map((x, j) => (j === i ? e.target.value : x)))}
                aria-label={translate('auto.mcp.external.argLabel', 'Argument {{n}}', { n })}
                className="h-8 max-w-sm font-mono"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                disabled={i === 0}
                aria-label={translate('auto.mcp.external.argUp', 'Move argument {{n}} up', { n })}
                onClick={() => move(i, -1)}
              >
                <ArrowUpIcon aria-hidden />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                disabled={i === args.length - 1}
                aria-label={translate('auto.mcp.external.argDown', 'Move argument {{n}} down', {
                  n
                })}
                onClick={() => move(i, 1)}
              >
                <ArrowDownIcon aria-hidden />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={translate('auto.mcp.external.argRemove', 'Remove argument {{n}}', {
                  n
                })}
                onClick={() => onChange(args.filter((_, j) => j !== i))}
              >
                <Trash2Icon aria-hidden />
              </Button>
            </div>
            {looksLikeSecretArg(a) ? (
              <p role="alert" className="text-xs text-destructive">
                {translate(
                  'auto.mcp.external.argSecretWarn',
                  "Don't put secrets in arguments — use an environment variable."
                )}
              </p>
            ) : null}
          </div>
        )
      })}
      <Button type="button" variant="outline" size="xs" onClick={() => onChange([...args, ''])}>
        <PlusIcon aria-hidden />
        {translate('auto.mcp.external.addArg', 'Add argument')}
      </Button>
    </div>
  )
}
