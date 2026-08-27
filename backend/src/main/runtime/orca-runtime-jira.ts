// frontend/src/main/runtime/orca-runtime-jira.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-045): Jira integration commands
// extracted from OrcaRuntimeService. Pure free-function wrappers with zero
// `this` dependency — no host interface needed, unlike every other domain
// split off this class. Sits immediately after Linear (TASK-BIGFILE-044) in
// the source, confirming both are separate, self-contained domains.
import type {
  JiraConnectArgs,
  JiraCreateIssueArgs,
  JiraIssueFilter,
  JiraIssueUpdate,
  JiraSiteSelection
} from '../../shared/types'
import {
  connect as connectJira,
  disconnect as disconnectJira,
  getStatus as getJiraStatus,
  selectSite as selectJiraSite,
  testConnection as testJiraConnection
} from '../jira/client'
import {
  addIssueComment as addJiraIssueComment,
  createIssue as createJiraIssue,
  getIssue as getJiraIssue,
  getIssueComments as getJiraIssueComments,
  getProjectStatusOrder as getJiraProjectStatusOrder,
  listAssignableUsers as listJiraAssignableUsers,
  listCreateFields as listJiraCreateFields,
  listIssueTypes as listJiraIssueTypes,
  listIssues as listJiraIssues,
  listPriorities as listJiraPriorities,
  listProjects as listJiraProjects,
  listTransitions as listJiraTransitions,
  searchIssues as searchJiraIssues,
  updateIssue as updateJiraIssue
} from '../jira/issues'

export class RuntimeJiraCommands {
  jiraConnect(args: JiraConnectArgs): ReturnType<typeof connectJira> {
    return connectJira(args)
  }

  jiraDisconnect(siteId?: string): { ok: true } {
    disconnectJira(siteId)
    return { ok: true }
  }

  jiraSelectSite(siteId: JiraSiteSelection): ReturnType<typeof getJiraStatus> {
    return selectJiraSite(siteId)
  }

  jiraStatus(): ReturnType<typeof getJiraStatus> {
    return getJiraStatus()
  }

  jiraTestConnection(siteId?: string): ReturnType<typeof testJiraConnection> {
    return testJiraConnection(siteId)
  }

  jiraSearchIssues(
    jql: string,
    limit = 30,
    siteId?: JiraSiteSelection
  ): ReturnType<typeof searchJiraIssues> {
    return searchJiraIssues(jql, Math.min(Math.max(1, limit), 100), siteId)
  }

  jiraListIssues(
    filter?: JiraIssueFilter,
    limit = 30,
    siteId?: JiraSiteSelection
  ): ReturnType<typeof listJiraIssues> {
    return listJiraIssues(filter, Math.min(Math.max(1, limit), 100), siteId)
  }

  jiraCreateIssue(args: JiraCreateIssueArgs): ReturnType<typeof createJiraIssue> {
    return createJiraIssue(args)
  }

  jiraGetIssue(key: string, siteId?: string): ReturnType<typeof getJiraIssue> {
    return getJiraIssue(key, siteId)
  }

  jiraUpdateIssue(
    key: string,
    updates: JiraIssueUpdate,
    siteId?: string
  ): ReturnType<typeof updateJiraIssue> {
    return updateJiraIssue(key, updates, siteId)
  }

  jiraAddIssueComment(
    key: string,
    body: string,
    siteId?: string
  ): ReturnType<typeof addJiraIssueComment> {
    return addJiraIssueComment(key, body, siteId)
  }

  jiraIssueComments(key: string, siteId?: string): ReturnType<typeof getJiraIssueComments> {
    return getJiraIssueComments(key, siteId)
  }

  jiraListProjects(siteId?: JiraSiteSelection): ReturnType<typeof listJiraProjects> {
    return listJiraProjects(siteId)
  }

  jiraListIssueTypes(
    projectIdOrKey: string,
    siteId?: string
  ): ReturnType<typeof listJiraIssueTypes> {
    return listJiraIssueTypes(projectIdOrKey, siteId)
  }

  jiraListCreateFields(
    projectIdOrKey: string,
    issueTypeId: string,
    siteId?: string
  ): ReturnType<typeof listJiraCreateFields> {
    return listJiraCreateFields(projectIdOrKey, issueTypeId, siteId)
  }

  jiraListPriorities(siteId?: string): ReturnType<typeof listJiraPriorities> {
    return listJiraPriorities(siteId)
  }

  jiraListAssignableUsers(
    key: string,
    query?: string,
    siteId?: string
  ): ReturnType<typeof listJiraAssignableUsers> {
    return listJiraAssignableUsers(key, query, siteId)
  }

  jiraListTransitions(key: string, siteId?: string): ReturnType<typeof listJiraTransitions> {
    return listJiraTransitions(key, siteId)
  }

  jiraGetProjectStatusOrder(
    projectKey: string,
    siteId?: string
  ): ReturnType<typeof getJiraProjectStatusOrder> {
    return getJiraProjectStatusOrder(projectKey, siteId)
  }
}
