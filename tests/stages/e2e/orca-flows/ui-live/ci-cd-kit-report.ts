/**
 * Milestone timings of the ci-cd-kit run and the short Vietnamese report written from what the run saw. Pure text: the facts are
 * collected by the steps, so the rendering is unit tested without a stack.
 */

export class Timeline {
  private readonly t0 = Date.now()
  readonly marks: { name: string; atMs: number }[] = []

  mark(name: string): void {
    this.marks.push({ name, atMs: Date.now() - this.t0 })
    console.log(`[ci-cd-kit] milestone ${this.format(name)}`)
  }

  private format(name: string): string {
    const m = this.marks.find((x) => x.name === name)
    return `${name} at ${minutes(m?.atMs ?? 0)}`
  }

  /** "name: +step (at total)" lines; a step is the time since the previous milestone. */
  lines(): string[] {
    return this.marks.map((m, i) => {
      const prev = i === 0 ? 0 : this.marks[i - 1].atMs
      return `${m.name}: +${minutes(m.atMs - prev)} (tại ${minutes(m.atMs)})`
    })
  }

  totalMs(): number {
    return this.marks.at(-1)?.atMs ?? 0
  }
}

export function minutes(ms: number): string {
  const s = Math.round(ms / 1000)
  return `${Math.floor(s / 60)}p${String(s % 60).padStart(2, '0')}s`
}

export type WorktreeFact = {
  path: string
  branch: string
  head: string
  /** The starting branch's old tip is an ancestor of this worktree's HEAD. */
  descendsFromBase: boolean
  mergeBase: string
  commitsAhead: number
  diffStat: string
  uncommitted: number
}

export type TaskFact = { type: string; title: string; status: string }

export type ReportFacts = {
  requestNumber: number | null
  requestId: string
  finalStatus: string
  baseBranch: string
  baseTip: string
  aiProposed: { type: string; size: string }
  questions: { prompt: string; answer: string }[]
  language: { isVietnamese: boolean; accentedPercent: number; sample: string } | null
  planWrittenWithoutGenerate: boolean | null
  planWaitMs: number | null
  documentTabs: string[]
  secretsOnScreen: string[]
  runAllUsed: boolean
  tasks: TaskFact[]
  worktrees: WorktreeFact[]
  staticChecks: string[]
  costLines: string[]
  findings: string[]
  timeline: string[]
  screenshotsDir: string
}

const yn = (v: boolean | null, yes: string, no: string, unknown = 'chưa xác định'): string =>
  v === null ? unknown : v ? yes : no

export function renderReportVi(f: ReportFacts): string {
  const out: string[] = []
  out.push(`# Báo cáo chạy thật: [TMS] CI-CD-KIT (BL-E07) trên dự án vnp-qa-platform`)
  out.push('')
  out.push(
    `Request #${f.requestNumber ?? '?'} (${f.requestId}), trạng thái cuối: **${f.finalStatus}**. Nhánh bắt đầu: \`${f.baseBranch}\` (tip \`${f.baseTip.slice(0, 10)}\`). Chạy qua giao diện, agent Claude thật.`
  )
  out.push('')
  out.push('## Thời gian từng bước')
  out.push(...f.timeline.map((l) => `- ${l}`))
  out.push('')
  out.push('## Phân loại và làm rõ')
  out.push(
    `- AI đề xuất: loại \`${f.aiProposed.type}\`, cỡ \`${f.aiProposed.size}\`; đã xác nhận change_request, M.`
  )
  for (const q of f.questions) {
    out.push(
      `- Hỏi: ${q.prompt.slice(0, 200)}  \n  Trả lời: ${q.answer.replace(/\s+/g, ' ').slice(0, 300)}`
    )
  }
  if (f.questions.length === 0) {
    out.push('- Không có câu hỏi làm rõ.')
  }
  out.push('')
  out.push('## Năm tính năng mới, thấy gì khi chạy thật')
  out.push(
    `1. Phân tích tiếng Việt (CR-REQ-046): ${f.language ? `${f.language.isVietnamese ? 'đúng tiếng Việt' : 'KHÔNG phải tiếng Việt'} (chữ có dấu ${f.language.accentedPercent.toFixed(1)}%). Trích: ${f.language.sample}` : 'chưa đo'}.`
  )
  out.push(
    `2. Plan tự sinh (CR-REQ-044): ${yn(f.planWrittenWithoutGenerate, 'Plan xuất hiện mà không bấm Generate', 'nút Generate đã hiện hoặc phải bấm')}${f.planWaitMs !== null ? `, chờ ${minutes(f.planWaitMs)}` : ''}.`
  )
  out.push(
    `3. Xem tài liệu của Plan (CR-REQ-045): các tab ${f.documentTabs.join(' | ') || 'không có'}; ${f.secretsOnScreen.length === 0 ? 'không thấy mẫu bí mật nào' : `PHÁT HIỆN MẪU BÍ MẬT: ${f.secretsOnScreen.join(', ')}`}.`
  )
  out.push(
    `4. "Approve and run all steps" (CR-REQ-043): ${yn(f.runAllUsed, 'đã dùng, chạy hết không cần duyệt từng phase', 'không dùng được')}.`
  )
  const allFromBase = f.worktrees.length > 0 && f.worktrees.every((w) => w.descendsFromBase)
  out.push(
    `5. Nhánh gốc của worktree (CR-REQ-039): ${f.worktrees.length === 0 ? 'chưa có worktree để kiểm' : allFromBase ? `cả ${f.worktrees.length} worktree đều là hậu duệ của tip ${f.baseTip.slice(0, 10)}` : 'CÓ worktree không xuất phát từ nhánh đã chọn'}.`
  )
  out.push('')
  out.push('## Task đã chạy')
  out.push(...f.tasks.map((t) => `- [${t.status}] ${t.type}: ${t.title}`))
  out.push('')
  out.push('## Worktree và nhánh mới (giữ lại để kiểm tra)')
  for (const w of f.worktrees) {
    out.push(
      `- \`${w.path}\` nhánh \`${w.branch || '(detached)'}\` HEAD \`${w.head.slice(0, 10)}\`: ${w.descendsFromBase ? 'hậu duệ của tip' : 'KHÔNG hậu duệ của tip'}, merge-base \`${w.mergeBase.slice(0, 10)}\`, ${w.commitsAhead} commit mới, ${w.uncommitted} file chưa commit`
    )
    if (w.diffStat) {
      out.push('  ```', ...w.diffStat.split('\n').map((l) => `  ${l}`), '  ```')
    }
  }
  out.push('')
  out.push('## Kiểm tra tĩnh các file agent viết')
  out.push(
    ...(f.staticChecks.length > 0
      ? f.staticChecks.map((l) => `- ${l}`)
      : ['- không có file để kiểm'])
  )
  out.push('')
  out.push('## Chi phí (từ số liệu của sản phẩm)')
  out.push(...(f.costLines.length > 0 ? f.costLines.map((l) => `- ${l}`) : ['- chưa lấy được']))
  out.push('')
  out.push('## Phát hiện / lỗi')
  out.push(...(f.findings.length > 0 ? f.findings.map((l) => `- ${l}`) : ['- không có']))
  out.push('')
  out.push(`Ảnh chụp từng bước: ${f.screenshotsDir}`)
  return `${out.join('\n')}\n`
}
