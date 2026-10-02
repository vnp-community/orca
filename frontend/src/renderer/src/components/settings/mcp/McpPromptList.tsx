import { CopyIcon, EyeIcon, LockIcon, PencilIcon, Trash2Icon } from 'lucide-react'
import type { McpPrompt } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

export type McpPromptListProps = {
  prompts: McpPrompt[]
  busyIds: ReadonlySet<string>
  onView: (p: McpPrompt) => void
  onEdit: (p: McpPrompt) => void
  onDuplicate: (p: McpPrompt) => void
  onDelete: (p: McpPrompt) => void
  /** Disables Duplicate when the custom-prompt limit is reached. */
  canCreate: boolean
}

function ArgumentsCell({ prompt }: { prompt: McpPrompt }): React.JSX.Element {
  const list = prompt.arguments.map((a) => (a.required ? `${a.name}*` : a.name)).join(', ')
  return (
    <TableCell title={list || undefined}>
      <span className="tabular-nums">{prompt.arguments.length}</span>
    </TableCell>
  )
}

export function McpPromptList({
  prompts,
  busyIds,
  onView,
  onEdit,
  onDuplicate,
  onDelete,
  canCreate
}: McpPromptListProps): React.JSX.Element {
  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          <tr>
            <TableHead>{translate('auto.mcp.prompts.col.name', 'Name')}</TableHead>
            <TableHead>{translate('auto.mcp.prompts.col.type', 'Type')}</TableHead>
            <TableHead>{translate('auto.mcp.prompts.col.description', 'Description')}</TableHead>
            <TableHead>{translate('auto.mcp.prompts.col.arguments', 'Arguments')}</TableHead>
            <TableHead>{translate('auto.mcp.prompts.col.version', 'Version')}</TableHead>
            <TableHead>{translate('auto.mcp.prompts.col.updated', 'Updated')}</TableHead>
            <TableHead className="text-right">
              <span className="sr-only">
                {translate('auto.mcp.prompts.col.actions', 'Actions')}
              </span>
            </TableHead>
          </tr>
        </TableHeader>
        <TableBody>
          {prompts.map((p) => {
            const busy = busyIds.has(p.id)
            return (
              <TableRow key={p.id}>
                <TableCell className="font-mono text-xs">{p.name}</TableCell>
                <TableCell>
                  {p.builtin ? (
                    <Badge variant="secondary">
                      <LockIcon aria-hidden />
                      {translate('auto.mcp.prompts.builtin', 'Built-in')}
                    </Badge>
                  ) : (
                    <Badge variant="outline">
                      {translate('auto.mcp.prompts.custom', 'Custom')}
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="max-w-64">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span tabIndex={0} className="block truncate">
                        {p.description}
                      </span>
                    </TooltipTrigger>
                    {p.description ? <TooltipContent>{p.description}</TooltipContent> : null}
                  </Tooltip>
                </TableCell>
                <ArgumentsCell prompt={p} />
                <TableCell className="tabular-nums">{`v${p.version}`}</TableCell>
                <TableCell>
                  <time dateTime={p.updatedAt} title={p.updatedAt}>
                    {formatMcpRelativeTime(p.updatedAt)}
                  </time>
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    {p.builtin ? (
                      <Button
                        variant="ghost"
                        size="xs"
                        aria-label={translate('auto.mcp.prompts.viewAria', 'View prompt {{name}}', {
                          name: p.name
                        })}
                        onClick={() => onView(p)}
                      >
                        <EyeIcon aria-hidden />
                        {translate('auto.mcp.prompts.view', 'View')}
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="xs"
                        disabled={busy}
                        aria-label={translate('auto.mcp.prompts.editAria', 'Edit prompt {{name}}', {
                          name: p.name
                        })}
                        onClick={() => onEdit(p)}
                      >
                        <PencilIcon aria-hidden />
                        {translate('auto.mcp.prompts.edit', 'Edit')}
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="xs"
                      disabled={busy || !canCreate}
                      aria-label={translate(
                        'auto.mcp.prompts.duplicateAria',
                        'Duplicate prompt {{name}}',
                        { name: p.name }
                      )}
                      onClick={() => onDuplicate(p)}
                    >
                      <CopyIcon aria-hidden />
                      {translate('auto.mcp.prompts.duplicate', 'Duplicate')}
                    </Button>
                    {p.builtin ? null : (
                      <Button
                        variant="ghost"
                        size="xs"
                        disabled={busy}
                        aria-label={translate(
                          'auto.mcp.prompts.deleteAria',
                          'Delete prompt {{name}}',
                          { name: p.name }
                        )}
                        onClick={() => onDelete(p)}
                      >
                        <Trash2Icon aria-hidden />
                        {translate('auto.mcp.prompts.delete', 'Delete')}
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </TooltipProvider>
  )
}
