// src/relay/agent-claude-tool-activity-text.ts
// Short, secret-safe one-line descriptions of a claude tool call and its result for
// the live activity feed. Only a fixed set of short fields is ever read: file contents
// and edit bodies (`content`, `old_string`, `new_string`, tool output of Read/Grep/Glob)
// are deliberately never shown.

const TOOL_DETAIL_CHARS = 160
const RESULT_DETAIL_CHARS = 120
const ERROR_DETAIL_CHARS = 160

export type JsonObject = Record<string, unknown>

export function isObject(value: unknown): value is JsonObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function safeSlice(text: string, max: number): string {
  let end = max
  const code = text.charCodeAt(end - 1)
  // Don't cut a surrogate pair in half.
  if (code >= 0xd800 && code <= 0xdbff) {
    end -= 1
  }
  return text.slice(0, end)
}

export function oneLine(text: string, max: number): string {
  const flat = text.replace(/\s+/g, ' ').trim()
  return flat.length > max ? `${safeSlice(flat, max - 1)}…` : flat
}

export function toPosix(path: string): string {
  return path.replace(/\\/g, '/')
}

// Why: show repo-relative paths, the absolute temp/worktree prefix is noise.
function relativeToCwd(path: string, prefixes: string[]): string {
  const posix = toPosix(path)
  for (const prefix of prefixes) {
    if (posix === prefix) {
      return '.'
    }
    if (posix.startsWith(`${prefix}/`)) {
      return posix.slice(prefix.length + 1)
    }
  }
  return posix
}

function stringField(input: JsonObject, key: string): string {
  const value = input[key]
  return typeof value === 'string' ? value : ''
}

// Why: query strings and fragments are where tokens travel.
function urlWithoutQuery(raw: string): string {
  return raw.split(/[?#]/)[0] ?? ''
}

export type ToolDescription = { detail: string; sep: ' ' | ': ' }
/** Redacts first, then shortens to one line (a cut secret must not survive as a fragment). */
export type ActivityCleaner = (text: string, max: number) => string

const PATH_TOOLS = new Set(['Read', 'Edit', 'Write', 'MultiEdit', 'NotebookEdit'])
const GENERIC_DETAIL_KEYS = [
  'file_path',
  'path',
  'pattern',
  'command',
  'url',
  'query',
  'description'
]

export function describeToolInput(
  name: string,
  input: JsonObject,
  prefixes: string[],
  clean: ActivityCleaner
): ToolDescription {
  const rel = (p: string): string => relativeToCwd(p, prefixes)
  if (PATH_TOOLS.has(name)) {
    const path = stringField(input, 'file_path') || stringField(input, 'notebook_path')
    return { detail: path ? clean(rel(path), TOOL_DETAIL_CHARS) : '', sep: ' ' }
  }
  switch (name) {
    case 'Glob': {
      const dir = stringField(input, 'path')
      return {
        detail: clean(
          `${stringField(input, 'pattern')}${dir ? ` in ${rel(dir)}` : ''}`,
          TOOL_DETAIL_CHARS
        ),
        sep: ' '
      }
    }
    case 'Grep': {
      const where = stringField(input, 'path') || stringField(input, 'glob')
      return {
        detail: clean(
          `${stringField(input, 'pattern')}${where ? ` in ${rel(where)}` : ''}`,
          TOOL_DETAIL_CHARS
        ),
        sep: ': '
      }
    }
    case 'Bash':
      return { detail: clean(stringField(input, 'command'), TOOL_DETAIL_CHARS), sep: ': ' }
    case 'WebFetch':
      return {
        detail: clean(urlWithoutQuery(stringField(input, 'url')), TOOL_DETAIL_CHARS),
        sep: ' '
      }
    case 'WebSearch':
      return { detail: clean(stringField(input, 'query'), 100), sep: ': ' }
    case 'Task':
    case 'Agent':
      return { detail: clean(stringField(input, 'description'), TOOL_DETAIL_CHARS), sep: ': ' }
    case 'TodoWrite':
      return { detail: Array.isArray(input.todos) ? `${input.todos.length} items` : '', sep: ': ' }
    default: {
      for (const key of GENERIC_DETAIL_KEYS) {
        const value = stringField(input, key)
        if (value) {
          return {
            detail: clean(key === 'url' ? urlWithoutQuery(value) : rel(value), TOOL_DETAIL_CHARS),
            sep: ': '
          }
        }
      }
      return { detail: '', sep: ' ' }
    }
  }
}

function toolResultText(content: unknown): string {
  if (typeof content === 'string') {
    return content
  }
  if (Array.isArray(content)) {
    return content
      .map((part) =>
        isObject(part) && part.type === 'text' && typeof part.text === 'string' ? part.text : ''
      )
      .join('\n')
  }
  return ''
}

function errorSummary(toolName: string, lines: string[], clean: ActivityCleaner): string {
  const first = (lines[0] ?? '').replace(/<\/?tool_use_error>/g, '')
  const last = lines.at(-1) ?? ''
  if (toolName === 'Bash' && lines.length > 1) {
    return `${clean(first, 40)} · ${clean(last, ERROR_DETAIL_CHARS - 40)}`
  }
  return clean(first || 'tool failed', ERROR_DETAIL_CHARS)
}

/** `ok · 3 lines`, `ok · <last Bash output line>` or `error: <reason>`; never the output itself for file tools. */
export function summarizeToolResult(
  toolName: string,
  block: JsonObject,
  clean: ActivityCleaner
): string {
  const lines = toolResultText(block.content)
    .split('\n')
    .filter((l) => l.trim().length > 0)
  if (block.is_error === true) {
    return `error: ${errorSummary(toolName, lines, clean)}`
  }
  if (toolName === 'Bash') {
    const last = clean(lines.at(-1) ?? '', RESULT_DETAIL_CHARS)
    return last ? `ok · ${last}` : 'ok'
  }
  if (PATH_TOOLS.has(toolName) && toolName !== 'Read') {
    return 'ok'
  }
  return `ok · ${lines.length} ${toolName === 'Glob' || toolName === 'Grep' ? 'results' : 'lines'}`
}
