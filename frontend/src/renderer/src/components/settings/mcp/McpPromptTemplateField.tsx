import { useRef } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  PROMPT_LIMITS,
  extractTemplateVariables,
  renderTemplatePreview
} from './mcp-prompt-template'
import { promptIssueMessage } from './mcp-prompt-issue-messages'
import type { PromptIssue } from './mcp-prompt-validation'

const braces = (name: string): string => `{{${name}}}`

export function McpPromptTemplateField({
  value,
  onChange,
  argNames,
  issues,
  readOnly
}: {
  value: string
  onChange: (next: string) => void
  argNames: string[]
  issues?: PromptIssue[]
  readOnly: boolean
}): React.JSX.Element {
  const ref = useRef<HTMLTextAreaElement>(null)
  const detected = extractTemplateVariables(value)
  const insert = (name: string): void => {
    const el = ref.current
    const start = el?.selectionStart ?? value.length
    const end = el?.selectionEnd ?? value.length
    onChange(value.slice(0, start) + braces(name) + value.slice(end))
    // Restore focus after React re-renders the controlled value.
    requestAnimationFrame(() => {
      el?.focus()
      const pos = start + braces(name).length
      el?.setSelectionRange(pos, pos)
    })
  }
  const invalid = Boolean(issues?.length)
  return (
    <div className="space-y-2">
      <Textarea
        ref={ref}
        id="mcp-prompt-template"
        value={value}
        readOnly={readOnly}
        rows={14}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={invalid ? true : undefined}
        aria-describedby="mcp-prompt-template-hint mcp-prompt-template-err"
        className="font-mono text-xs"
      />
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span id="mcp-prompt-template-hint">
          {translate(
            'auto.mcp.prompts.templateHint',
            'Use {{example}} to insert an argument. Conditions and loops are not supported.',
            { example: braces('name') }
          )}
        </span>
        <span>{`${value.length} / ${PROMPT_LIMITS.template}`}</span>
      </div>
      {readOnly ? null : (
        <div className="flex flex-wrap items-center gap-1.5">
          {argNames
            .filter((n) => n !== '')
            .map((n) => (
              <Button key={n} type="button" variant="outline" size="xs" onClick={() => insert(n)}>
                {braces(n)}
              </Button>
            ))}
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        {translate('auto.mcp.prompts.detected', 'Detected variables:')}{' '}
        {detected.length ? detected.map(braces).join(', ') : '—'}
      </p>
      <div
        id="mcp-prompt-template-err"
        role="alert"
        className="space-y-0.5 text-xs text-destructive"
      >
        {issues?.map((issue, i) => (
          <p key={i}>{promptIssueMessage(issue)}</p>
        ))}
      </div>
      <div>
        <p className="text-xs font-medium">
          {translate('auto.mcp.prompts.preview', 'Preview (placeholders shown as ⟨name⟩)')}
        </p>
        {/* Plain text only: template content is never parsed as HTML or markdown. */}
        <pre className="mt-1 max-h-48 overflow-auto rounded-md bg-muted p-2 text-xs break-words whitespace-pre-wrap">
          {renderTemplatePreview(value, {})}
        </pre>
      </div>
    </div>
  )
}
