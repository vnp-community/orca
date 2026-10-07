/**
 * Request Status Presentation — CR-REQ-018-05
 *
 * Maps request/approval enums → {icon, toneClass, labelKey} for badges and timelines.
 * Pure TypeScript: no React imports.
 *
 * Tone classes use CSS tokens from STYLEGUIDE.md:
 *   status-success, destructive, primary, muted-foreground
 * NOT raw hex or Tailwind colour primitives.
 *
 * @module components/request/request-status-presentation
 */

import type { RequestStatus, RequestType, ApprovalStatus, RequestSourceProvider } from '../../../../shared/request-types'

// ---------------------------------------------------------------------------
// RequestStatus
// ---------------------------------------------------------------------------

export type StatusPresentation = {
  /** lucide icon name (caller imports from 'lucide-react') */
  iconName: string
  /** CSS token class — one of: status-success, destructive, primary, muted-foreground */
  toneClass: 'status-success' | 'destructive' | 'primary' | 'muted-foreground'
  /** i18n key prefix: auto.components.request.RequestStatus.<value> */
  labelKey: string
}

const STATUS_PRESENTATION: Record<RequestStatus, StatusPresentation> = {
  submitted: {
    iconName: 'SendHorizontal',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestStatus.submitted'
  },
  classifying: {
    iconName: 'ScanSearch',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.classifying'
  },
  awaiting_type_confirmation: {
    iconName: 'CircleHelp',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.awaiting_type_confirmation'
  },
  analyzing: {
    iconName: 'BrainCircuit',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.analyzing'
  },
  awaiting_analysis_approval: {
    iconName: 'CircleCheck',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.awaiting_analysis_approval'
  },
  awaiting_information: {
    iconName: 'MessageCircleQuestion',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestStatus.awaiting_information'
  },
  planning: {
    iconName: 'ListTodo',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.planning'
  },
  awaiting_plan_approval: {
    iconName: 'ClipboardCheck',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.awaiting_plan_approval'
  },
  executing: {
    iconName: 'Cpu',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestStatus.executing'
  },
  completed: {
    iconName: 'CheckCircle2',
    toneClass: 'status-success',
    labelKey: 'auto.components.request.RequestStatus.completed'
  },
  cancelled: {
    iconName: 'Ban',
    toneClass: 'destructive',
    labelKey: 'auto.components.request.RequestStatus.cancelled'
  },
  request_backlog: {
    iconName: 'Inbox',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestStatus.request_backlog'
  },
  unknown: {
    iconName: 'Circle',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestStatus.unknown'
  }
}

export function getRequestStatusPresentation(status: RequestStatus): StatusPresentation {
  return STATUS_PRESENTATION[status] ?? STATUS_PRESENTATION.unknown
}

// ---------------------------------------------------------------------------
// RequestType
// ---------------------------------------------------------------------------

export type TypePresentation = {
  iconName: string
  toneClass: 'status-success' | 'destructive' | 'primary' | 'muted-foreground'
  labelKey: string
  descriptionKey: string
}

const TYPE_PRESENTATION: Record<RequestType, TypePresentation> = {
  bug: {
    iconName: 'Bug',
    toneClass: 'destructive',
    labelKey: 'auto.components.request.RequestType.bug.label',
    descriptionKey: 'auto.components.request.RequestType.bug.description'
  },
  task: {
    iconName: 'CheckSquare',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestType.task.label',
    descriptionKey: 'auto.components.request.RequestType.task.description'
  },
  docs: {
    iconName: 'BookOpen',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestType.docs.label',
    descriptionKey: 'auto.components.request.RequestType.docs.description'
  },
  question: {
    iconName: 'MessageCircleQuestion',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestType.question.label',
    descriptionKey: 'auto.components.request.RequestType.question.description'
  },
  hotfix: {
    iconName: 'Zap',
    toneClass: 'destructive',
    labelKey: 'auto.components.request.RequestType.hotfix.label',
    descriptionKey: 'auto.components.request.RequestType.hotfix.description'
  },
  security: {
    iconName: 'ShieldAlert',
    toneClass: 'destructive',
    labelKey: 'auto.components.request.RequestType.security.label',
    descriptionKey: 'auto.components.request.RequestType.security.description'
  },
  ops_request: {
    iconName: 'Server',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestType.ops_request.label',
    descriptionKey: 'auto.components.request.RequestType.ops_request.description'
  },
  change_request: {
    iconName: 'GitPullRequest',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestType.change_request.label',
    descriptionKey: 'auto.components.request.RequestType.change_request.description'
  },
  refactor: {
    iconName: 'RefreshCw',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestType.refactor.label',
    descriptionKey: 'auto.components.request.RequestType.refactor.description'
  },
  spike: {
    iconName: 'FlaskConical',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestType.spike.label',
    descriptionKey: 'auto.components.request.RequestType.spike.description'
  },
  performance: {
    iconName: 'Gauge',
    toneClass: 'primary',
    labelKey: 'auto.components.request.RequestType.performance.label',
    descriptionKey: 'auto.components.request.RequestType.performance.description'
  },
  unknown: {
    iconName: 'Circle',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.RequestType.unknown.label',
    descriptionKey: 'auto.components.request.RequestType.unknown.description'
  }
}

export function getRequestTypePresentation(type: RequestType): TypePresentation {
  return TYPE_PRESENTATION[type] ?? TYPE_PRESENTATION.unknown
}

// ---------------------------------------------------------------------------
// ApprovalStatus
// ---------------------------------------------------------------------------

export type ApprovalPresentation = {
  iconName: string
  toneClass: 'status-success' | 'destructive' | 'primary' | 'muted-foreground'
  labelKey: string
}

const APPROVAL_PRESENTATION: Record<ApprovalStatus, ApprovalPresentation> = {
  pending: {
    iconName: 'Clock',
    toneClass: 'primary',
    labelKey: 'auto.components.request.ApprovalStatus.pending'
  },
  approved: {
    iconName: 'CheckCircle2',
    toneClass: 'status-success',
    labelKey: 'auto.components.request.ApprovalStatus.approved'
  },
  rejected: {
    iconName: 'XCircle',
    toneClass: 'destructive',
    labelKey: 'auto.components.request.ApprovalStatus.rejected'
  },
  expired: {
    iconName: 'Timer',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.ApprovalStatus.expired'
  },
  unknown: {
    iconName: 'Circle',
    toneClass: 'muted-foreground',
    labelKey: 'auto.components.request.ApprovalStatus.unknown'
  }
}

export function getApprovalPresentation(status: ApprovalStatus): ApprovalPresentation {
  return APPROVAL_PRESENTATION[status] ?? APPROVAL_PRESENTATION.unknown
}

// ---------------------------------------------------------------------------
// RequestSourceProvider
// ---------------------------------------------------------------------------

export type SourcePresentation = {
  iconName: string
  labelKey: string
}

const SOURCE_PRESENTATION: Record<RequestSourceProvider, SourcePresentation> = {
  jira: { iconName: 'ExternalLink', labelKey: 'auto.components.request.RequestSource.jira' },
  github: { iconName: 'Github', labelKey: 'auto.components.request.RequestSource.github' },
  gitlab: { iconName: 'GitBranch', labelKey: 'auto.components.request.RequestSource.gitlab' },
  linear: { iconName: 'Layout', labelKey: 'auto.components.request.RequestSource.linear' },
  mcp: { iconName: 'Cpu', labelKey: 'auto.components.request.RequestSource.mcp' },
  manual: { iconName: 'Pencil', labelKey: 'auto.components.request.RequestSource.manual' },
  webhook: { iconName: 'Webhook', labelKey: 'auto.components.request.RequestSource.webhook' },
  unknown: { iconName: 'Circle', labelKey: 'auto.components.request.RequestSource.unknown' }
}

export function getSourcePresentation(provider: RequestSourceProvider): SourcePresentation {
  return SOURCE_PRESENTATION[provider] ?? SOURCE_PRESENTATION.unknown
}
