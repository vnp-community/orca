import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { PROMPT_LIMITS } from './mcp-prompt-template'
import { promptIssueMessage } from './mcp-prompt-issue-messages'
import type { PromptDraft, PromptFieldErrors } from './mcp-prompt-validation'

type Arg = PromptDraft['arguments'][number]

export function McpPromptArgumentsEditor({
  args,
  onChange,
  errors,
  readOnly
}: {
  args: Arg[]
  onChange: (next: Arg[]) => void
  errors?: PromptFieldErrors['arguments']
  readOnly: boolean
}): React.JSX.Element {
  const patch = (i: number, p: Partial<Arg>): void =>
    onChange(args.map((a, j) => (j === i ? { ...a, ...p } : a)))
  const move = (i: number, by: -1 | 1): void => {
    const next = [...args]
    ;[next[i], next[i + by]] = [next[i + by], next[i]]
    onChange(next)
  }
  return (
    <div className="space-y-2">
      {args.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {translate('auto.mcp.prompts.noArgs', 'No arguments.')}
        </p>
      ) : null}
      {args.map((a, i) => {
        const err = errors?.[i]?.name
        const errId = `mcp-prompt-arg-${i}-err`
        const n = i + 1
        return (
          <div key={i} className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <Input
                value={a.name}
                readOnly={readOnly}
                onChange={(e) => patch(i, { name: e.target.value })}
                aria-label={translate('auto.mcp.prompts.argName', 'Argument {{n}} name', { n })}
                aria-invalid={err ? true : undefined}
                aria-describedby={err ? errId : undefined}
                placeholder={translate('auto.mcp.prompts.argNamePlaceholder', 'name')}
                className="h-8 w-36 font-mono"
              />
              <Input
                value={a.description}
                readOnly={readOnly}
                onChange={(e) => patch(i, { description: e.target.value })}
                aria-label={translate(
                  'auto.mcp.prompts.argDescription',
                  'Argument {{n}} description',
                  { n }
                )}
                placeholder={translate('auto.mcp.prompts.argDescriptionPlaceholder', 'Description')}
                className="h-8 min-w-40 flex-1"
              />
              <label className="flex items-center gap-1.5 text-sm">
                <Checkbox
                  checked={a.required}
                  disabled={readOnly}
                  onCheckedChange={(c) => patch(i, { required: c === true })}
                />
                {translate('auto.mcp.prompts.required', 'Required')}
              </label>
              {readOnly ? null : (
                <>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    disabled={i === 0}
                    aria-label={translate('auto.mcp.prompts.moveUp', 'Move argument up')}
                    onClick={() => move(i, -1)}
                  >
                    <ArrowUpIcon aria-hidden />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    disabled={i === args.length - 1}
                    aria-label={translate('auto.mcp.prompts.moveDown', 'Move argument down')}
                    onClick={() => move(i, 1)}
                  >
                    <ArrowDownIcon aria-hidden />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={translate('auto.mcp.prompts.removeArg', 'Remove argument {{n}}', {
                      n
                    })}
                    onClick={() => onChange(args.filter((_, j) => j !== i))}
                  >
                    <Trash2Icon aria-hidden />
                  </Button>
                </>
              )}
            </div>
            {err ? (
              <p id={errId} role="alert" className="text-xs text-destructive">
                {promptIssueMessage(err)}
              </p>
            ) : null}
          </div>
        )
      })}
      {readOnly ? null : (
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={args.length >= PROMPT_LIMITS.args}
          onClick={() => onChange([...args, { name: '', description: '', required: false }])}
        >
          <PlusIcon aria-hidden />
          {translate('auto.mcp.prompts.addArg', 'Add argument')}
        </Button>
      )}
    </div>
  )
}
