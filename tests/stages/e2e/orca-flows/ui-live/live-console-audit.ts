/**
 * Every browser console error/warning of a live run, with the step that was running, classified. A known-noise rule needs
 * a reason; an entry that matches none is a finding and fails the spec, so a new console error cannot slip by unseen.
 */
import type { ConsoleMessage, Page, TestInfo } from '@playwright/test'

export type ConsoleEntry = { type: string; text: string; step: string; url: string }
export type ConsoleVerdict = { entry: ConsoleEntry; rule: string; known: boolean; reason: string }

type NoiseRule = { id: string; match: RegExp; reason: string; url?: RegExp }

// Why: each rule is a cause already investigated (TASK-FE-E2E-010 has the evidence); a new entry must be explained before it is added here.
const NOISE_RULES: NoiseRule[] = [
  {
    id: 'auth-me-before-sign-in',
    match: /Failed to load resource: the server responded with a status of 401/,
    url: /^\/auth\/me$/,
    reason: '/auth/me answers 401 before the sign-in form is submitted, by design'
  },
  {
    id: 'sidebar-worktrees-of-stale-projects',
    match:
      /^(Failed to fetch worktrees for repo [0-9a-f-]{36}|Failed to list runtime detected worktrees|\[web-preload-api\] worktree\.detectedList failed for repo [0-9a-f-]{36}): .*(WORKTREE_(DETECT|RESOLVE)_FAILED|GITGATEWAY_(DEV_SERVER_UNREACHABLE|REPO_HOST_UNBOUND|HOST_CHECK_FAILED))/,
    reason:
      'the sidebar lists worktrees of every project; older projects on the stack point at paths git-gateway cannot open (WORKTREE_DETECT_FAILED) or at dev servers that are gone or disconnected (WORKTREE_RESOLVE_FAILED, and since V2-035 GITGATEWAY_DEV_SERVER_UNREACHABLE), or at repos saved before host binding that have no dev server (GITGATEWAY_REPO_HOST_UNBOUND); data left by earlier runs, not Request code. The sidebar retries them 6 to 14 times per load'
  },
  {
    id: 'scm-empty-branch-g5b',
    match: /SCM_EMPTY_BRANCH: head_branch is required/,
    reason:
      'G5b (TASK-FE-E2E-011): the Source Control panel sends branch/base/repo to hostedReview.* and github.prForBranch while the gateway reads headBranch/baseBranch, so head_branch is empty on every worktree; known, needs a design decision, unrelated to the task flow'
  },
  {
    id: 'diff-view-crash-g5d',
    match:
      /Cannot read properties of undefined \(reading 'length'\)|\[terminal\.workbench\] render crash contained by boundary/,
    reason:
      'G5d (TASK-FE-E2E-011): opening a changed file in Source Control renders a diff from a git.diff reply that arrives in the wrong shape (snake_case proto fields, empty diff through the relay, G5e); the crash is contained by the boundary. Known, listed as a known-gap annotation by the monitoring spec'
  },
  {
    id: 'webgl-gpu-stall-in-headless-chromium',
    match: /^\[\.WebGL-[0-9a-fx]+\]GL Driver Message .*GPU stall due to ReadPixels/,
    reason:
      'the terminal canvas reads pixels back under the software GL of headless Chromium; a browser performance notice, not an app error'
  },
  {
    id: 'radix-dialog-description-add-project',
    match:
      /^Warning: Missing `Description` or `aria-describedby=\{undefined\}` for \{DialogContent\}\./,
    reason:
      'a dialog of the Add Project flow (AddRepoDialogChrome.tsx wraps it in DialogContent without a description or aria-describedby), so Radix warns on every open; an accessibility warning that predates the add-project fixes, to be fixed in the frontend'
  },
  {
    id: 'scan-nested-agent-method-missing',
    match: /^Failed to scan nested repos: .*PROJECT_SCAN_NESTED_FAILED/,
    reason:
      'project-service ScanNested calls fs.scanNestedRepos on the dev-server agent, which does not implement it (infra-fleet log: "Method not found: fs.scanNestedRepos"; agent/src has no such handler); nested-repo import never works, Add Project carries on without it. Backend/agent gap that predates the add-project fixes (TASK-FE-E2E-027)'
  },
  {
    id: 'profiles-channel-not-in-gateway',
    match:
      /^Failed to check project presence in other profiles: .*orcaProfiles\.findProjectProfiles.*not yet implemented/,
    reason:
      'orcaProfiles.findProjectProfiles is on the backend-go frontend-compatibility coverage table as not implemented; Add Project only logs it. Backend gap that predates the add-project fixes (TASK-FE-E2E-027)'
  },
  {
    id: 'work-items-github-remote-lookup',
    match:
      /^\[workItems\] dropping issues-side error with no resolved source: .*no GitHub remote configured.*WORKTREE_REPO_NOT_FOUND/,
    reason:
      'right after a worktree is created on a repo that has a GitHub remote, the work-items panel looks up the remote and git-gateway answers WORKTREE_REPO_NOT_FOUND (repo does not exist); the panel drops the error and shows nothing. Seen only on a repo with a real remote (TASK-FE-E2E-028); a backlog item to look at, not Request code'
  },
  {
    id: 'trace-snapshot-script-in-sandboxed-frame',
    match:
      /^Blocked script execution in 'about:srcdoc' because the document's frame is sandboxed and the 'allow-scripts' permission is not set/,
    reason:
      "Playwright's trace snapshotter (tracing.start with snapshots) injects a script into every frame; a review drawing is shown in an iframe with an empty sandbox on purpose, so the injection is refused. Reproduced with a bare sandboxed srcdoc: the message appears with snapshots on and not with snapshots off, and no drawing holds a script"
  },
  {
    id: 'decision-channel-not-served',
    match: /decision\.[a-zA-Z]+.*not yet implemented/,
    reason:
      'decision.* is on the CONTRACT allowlist of channels the gateway does not serve yet (FE-E2E-001)'
  }
]

export class ConsoleAudit {
  readonly entries: ConsoleEntry[] = []

  constructor(
    page: Page,
    private readonly currentStep: () => string
  ) {
    page.on('console', (m: ConsoleMessage) => {
      const type = m.type()
      if (type === 'error' || type === 'warning') {
        this.entries.push({
          type,
          text: m.text().replace(/\s+/g, ' ').slice(0, 400),
          step: this.currentStep(),
          url: m
            .location()
            .url.replace(/^https?:\/\/[^/]+/, '')
            .slice(0, 120)
        })
      }
    })
    page.on('pageerror', (e) =>
      this.entries.push({
        type: 'pageerror',
        text: String(e).replace(/\s+/g, ' ').slice(0, 400),
        step: this.currentStep(),
        url: ''
      })
    )
  }

  classify(): ConsoleVerdict[] {
    return this.entries.map((entry) => {
      const rule = NOISE_RULES.find(
        (r) => r.match.test(entry.text) && (!r.url || r.url.test(entry.url))
      )
      return rule
        ? { entry, rule: rule.id, known: true, reason: rule.reason }
        : {
            entry,
            rule: 'unclassified',
            known: false,
            reason: 'no allowlist rule explains this entry'
          }
    })
  }

  /** The report file next to the screenshots plus one log line per entry (what the spec asks to see in the output). */
  report(testInfo: TestInfo, slug: string): ConsoleVerdict[] {
    const verdicts = this.classify()
    for (const v of verdicts) {
      console.log(
        `[console:${slug}] ${v.known ? 'noise' : 'FINDING'} ${v.entry.type} @ "${v.entry.step}" ${v.entry.url}: ${v.entry.text}`
      )
    }
    testInfo.annotations.push({
      type: 'console-audit',
      description: `${verdicts.length} entries, ${verdicts.filter((v) => !v.known).length} unclassified`
    })
    return verdicts
  }
}
