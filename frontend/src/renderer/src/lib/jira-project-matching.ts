export const JIRA_PROJECT_KEY_PATTERN = /^[A-Z][A-Z0-9_]{1,19}$/

type JiraMappedProject = { id: string; jiraProjectKey?: string; jiraSiteId?: string }

export function getJiraIssueKeyPrefix(issueKey: string | null | undefined): string | null {
  const match = /^([A-Za-z][A-Za-z0-9_]*)-\d+$/.exec(issueKey?.trim() ?? '')
  return match ? match[1].toUpperCase() : null
}

// Why: a key prefix can be reused across Jira sites, so a site match wins; with
// several equally good candidates we refuse to guess and let the user pick.
export function matchProjectForJiraIssue<T extends JiraMappedProject>(
  projects: readonly T[],
  issueKey: string | null | undefined,
  siteId?: string | null
): T | null {
  const prefix = getJiraIssueKeyPrefix(issueKey)
  if (!prefix) {
    return null
  }
  const byKey = projects.filter((p) => p.jiraProjectKey?.trim().toUpperCase() === prefix)
  if (siteId) {
    const bySite = byKey.filter((p) => p.jiraSiteId === siteId)
    if (bySite.length > 0) {
      return bySite.length === 1 ? bySite[0] : null
    }
    // Projects mapped to a different site are not a match; site-less ones are.
    const siteless = byKey.filter((p) => !p.jiraSiteId)
    return siteless.length === 1 ? siteless[0] : null
  }
  return byKey.length === 1 ? byKey[0] : null
}
