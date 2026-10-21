/**
 * The backlog item BL-E07 of the user's project vnp-qa-platform as the text of a Request, and the answers to the readiness
 * Clarification built from it. Pure: the file is read by the caller, so everything here is unit tested without a stack.
 */
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

export const CICDKIT_REPO =
  process.env.ORCA_E2E_CICDKIT_REPO ||
  '/Users/datbeohbbh/Desktop/work/vnpay/agentic-app/vnp-qa-platform'
export const CICDKIT_PROJECT_NAME = 'vnp-qa-platform'
export const CICDKIT_BRANCH = 'claude/tms-bug-triage-report-09f310'
// Why: the same flow serves any backlog item of the project; the defaults are the BL-E07 run.
export const CICDKIT_TITLE = process.env.ORCA_E2E_REQUEST_TITLE || '[TMS] CI-CD-KIT (BL-E07)'
export const BACKLOG_FILE = process.env.ORCA_E2E_BACKLOG_FILE || 'docs/backlog/BL-E07_ci-cd-kit.md'
/** The unhurried pointer used where no sentence of the backlog answers a question. */
export const POINTER_ANSWER = `See ${/(BL-[A-Z]\d+)/.exec(BACKLOG_FILE)?.[1] ?? 'the backlog item'} acceptance criteria`

export type BacklogItem = {
  id: string
  title: string
  priority: string
  milestone: string
  size: string
  /** Section heading (without "## ") -> its text. */
  sections: Record<string, string>
}

function frontmatterValue(front: string, key: string): string {
  const m = new RegExp(`^${key}:\\s*(.*)$`, 'm').exec(front)
  return (m?.[1] ?? '').trim().replace(/^["']|["']$/g, '')
}

export function parseBacklog(markdown: string): BacklogItem {
  const m = /^---\n([\s\S]*?)\n---\n([\s\S]*)$/.exec(markdown.replace(/\r\n/g, '\n'))
  if (!m) {
    throw new Error('the backlog file has no front matter')
  }
  const sections: Record<string, string> = {}
  let current = ''
  for (const line of m[2].split('\n')) {
    const heading = /^##\s+(.+?)\s*$/.exec(line)
    if (heading) {
      current = heading[1]
      sections[current] = ''
    } else if (current) {
      sections[current] += `${line}\n`
    }
  }
  for (const k of Object.keys(sections)) {
    sections[k] = sections[k].trim()
  }
  return {
    id: frontmatterValue(m[1], 'id'),
    title: frontmatterValue(m[1], 'title'),
    priority: frontmatterValue(m[1], 'priority'),
    milestone: frontmatterValue(m[1], 'milestone'),
    size: frontmatterValue(m[1], 'size'),
    sections
  }
}

export function readBacklog(repo = CICDKIT_REPO): BacklogItem {
  return parseBacklog(readFileSync(join(repo, BACKLOG_FILE), 'utf8'))
}

/** The first section whose heading starts with one of the words (headings carry a dash and a gloss: "Bối cảnh — vì sao cần"). */
function section(item: BacklogItem, ...starts: string[]): string {
  const key = Object.keys(item.sections).find((k) => starts.some((s) => k.startsWith(s)))
  return key ? item.sections[key] : ''
}

/** What the agents may not do on this machine while they work; the owner's stack of the same repo is running here. */
export const EXECUTION_LIMITS = [
  'Chỉ ghi file trong worktree của task; không `git push`, không đổi remote hay cấu hình git.',
  'Không khởi động hay dừng docker/docker compose, không deploy, không gọi dịch vụ đang chạy trên máy; không chạy bộ e2e đầy đủ của repo.',
  'Các e2e và pipeline mẫu được viết nhưng chỉ kiểm bằng cách chạy cục bộ không cần dịch vụ ngoài (giả lập API bằng script, parse YAML, `bash -n`); lệnh Verify của task phải chạy được như vậy.'
]

/** The Request text: goal, scope and acceptance criteria of the item, said to come from it, with the limits above. */
export function requestBody(item: BacklogItem): string {
  const parts = [
    `Nguồn: backlog ${item.id} (${BACKLOG_FILE} của repo vnp-qa-platform), ưu tiên ${item.priority}, mốc ${item.milestone}, cỡ ${item.size}.`,
    `# ${item.id} — ${item.title}`,
    `## Mục tiêu\n${section(item, 'Bối cảnh')}`,
    `## Hiện trạng\n${section(item, 'Hiện trạng')}`,
    `## Phạm vi công việc\n${section(item, 'Phạm vi')}`,
    `## Tiêu chí nghiệm thu\n${section(item, 'Tiêu chí')}`,
    `## Nguồn tham chiếu\n${section(item, 'Nguồn')}`,
    `## Giới hạn khi thực thi\n${EXECUTION_LIMITS.map((l) => `- ${l}`).join('\n')}`
  ]
  return parts.join('\n\n')
}

const strip = (text: string): string =>
  text
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase()

const words = (text: string): Set<string> => new Set(strip(text).match(/[a-z0-9]{4,}/g) ?? [])

/** Question topics -> the sections of the item that answer them (Vietnamese and English cues, diacritics dropped). */
const TOPICS: { cue: RegExp; sections: string[] }[] = [
  {
    cue: /acceptance|criteri|nghiem thu|tieu chi|definition of done|done when|success/,
    sections: ['Tiêu chí']
  },
  {
    cue: /scope|pham vi|include|exclude|out of scope|deliverable|work item/,
    sections: ['Phạm vi']
  },
  {
    cue: /why|goal|purpose|benefit|business|value|muc tieu|boi canh|gia tri|loi ich|mang lai|motivation|problem/,
    sections: ['Bối cảnh', 'Hiện trạng']
  },
  { cue: /current|existing|today|hien trang|already|status/, sections: ['Hiện trạng'] },
  { cue: /depend|prerequisite|phu thuoc|block/, sections: ['Phụ thuộc'] },
  { cue: /source|reference|spec|document|nguon|where/, sections: ['Nguồn'] }
]

const FACTS = (item: BacklogItem): string =>
  `${item.id}: ưu tiên ${item.priority}, mốc ${item.milestone}, cỡ ${item.size} (tối đa một người-tuần).`

/** The lines of the item (bullets and sentences) that share the most words with a question, best first. */
function bestLines(item: BacklogItem, query: string, take: number): string[] {
  const asked = words(query)
  const scored: { line: string; score: number }[] = []
  for (const [name, text] of Object.entries(item.sections)) {
    if (name.startsWith('Nguồn')) {
      continue
    }
    for (const raw of text.split('\n')) {
      const line = raw.replace(/^[-*]\s*(\[[ x~c]\]\s*)?/, '').trim()
      if (line.length < 12) {
        continue
      }
      let score = 0
      for (const w of words(line)) {
        if (asked.has(w)) {
          score++
        }
      }
      if (score >= 2) {
        scored.push({ line, score })
      }
    }
  }
  return scored
    .sort((a, b) => b.score - a.score)
    .slice(0, take)
    .map((s) => s.line)
}

/** A written answer to a free-text question: the section the question is about, else the best matching lines, else the pointer. */
export function textAnswer(item: BacklogItem, prompt: string, reason = ''): string {
  const asked = strip(`${prompt} ${reason}`)
  if (/priorit|uu tien|timeline|deadline|when|bao lau|effort|size|estimate|milestone/.test(asked)) {
    return FACTS(item)
  }
  for (const topic of TOPICS) {
    if (topic.cue.test(asked)) {
      const text = topic.sections
        .map((s) => section(item, s))
        .filter(Boolean)
        .join('\n')
      if (text) {
        return `Theo ${item.id}:\n${text}`
      }
    }
  }
  const lines = bestLines(item, `${prompt} ${reason}`, 3)
  return lines.length > 0
    ? `Theo ${item.id}:\n${lines.map((l) => `- ${l}`).join('\n')}`
    : POINTER_ANSWER
}

/** The option of a choice question that shares the most words with the item; the first when none does. */
export function pickOption(item: BacklogItem, labels: string[]): number {
  if (labels.length === 0) {
    return -1
  }
  const pool = words(Object.values(item.sections).join(' '))
  let best = 0
  let bestScore = -1
  labels.forEach((label, i) => {
    let score = 0
    for (const w of words(label)) {
      if (pool.has(w)) {
        score++
      }
    }
    if (score > bestScore) {
      best = i
      bestScore = score
    }
  })
  return best
}
