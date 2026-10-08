/**
 * quality-scorecard-copy.ts — FE-CV-TASK-087-04
 *
 * English copy of the quality scorecard, run control and lens state screens. Keys are read by
 * name; the locale coverage test enumerates this table. Wording rule (contract §4.7): `unknown`
 * is "not enough data to conclude", never a pass; nothing here says "safe" or "clean".
 *
 * @module components/review-map/quality/quality-scorecard-copy
 */

import { createQualityCopy } from './quality-copy-factory'

export const QUALITY_SCORECARD_COPY = {
  // Lens and blocks
  'lens.title': 'Quality',
  'lens.findingsLink': 'View {{count}} findings',
  'lens.findingsLinkNone': 'View findings',
  'blocks.coverage': 'Test coverage',
  'blocks.trend': 'Trend by turn',
  'blocks.hotspot': 'Hotspots',
  'blocks.dependency': 'Dependencies',
  'blocks.loading': 'Loading',
  // Verdict
  'verdict.pass': 'The checks of profile {{profile}} all passed',
  'verdict.warn': 'Has warnings',
  'verdict.fail': 'Did not pass',
  'verdict.unknown': 'Not enough data to conclude',
  'verdict.unknownNoReason': 'The backend gave no reason',
  'verdict.profileMissing': 'unnamed profile',
  'mode.inform': 'Inform mode: this does not block creating a review',
  'mode.block': 'Configured to block; this interface only warns',
  'mode.unknown': 'The gate mode is not known',
  // Reasons
  'reasons.title': 'Checks behind this verdict',
  'reasons.none': 'The backend returned no per-check reasons',
  'reason.pass': 'Passed',
  'reason.warn': 'Warning',
  'reason.fail': 'Failed',
  'reason.unknown': 'Unknown',
  'reason.values': 'observed {{observed}}, threshold {{threshold}}',
  'reason.waived': '{{count}} waived',
  'reason.show': 'Show findings',
  'reason.noFilter': 'Findings cannot be filtered for this check',
  // Steps
  'steps.title': 'Steps that ran ({{count}})',
  'steps.none': 'No steps were recorded for this run',
  'step.passed': 'Passed',
  'step.findings': 'Has findings',
  'step.failed': 'Failed',
  'step.timeout': 'Timed out',
  'step.cancelled': 'Cancelled',
  'step.skipped': 'Skipped',
  'step.env_not_ready': 'Environment not ready',
  'step.unknown': 'Unknown',
  'step.failure.format_drift': 'Output format changed',
  'step.failure.output_too_large': 'Output too large',
  'step.failure.exit_unexpected': 'Unexpected exit',
  'step.failure.parser_error': 'Output could not be parsed',
  'step.failure.env': 'Environment problem',
  'step.noResult':
    'This step did not complete, so it contributed no findings and its result is unknown',
  'step.counts': '{{errors}} errors, {{warnings}} warnings, {{infos}} info',
  'step.duration': '{{seconds}} s',
  'step.truncated': 'Output truncated',
  'step.outside': '{{count}} outside the changed scope',
  // Provenance and staleness
  'source.local': 'Local',
  'source.ci': 'CI',
  'source.unknown': 'Unknown source',
  'provenance.line': '{{source}} · ran {{time}} · HEAD {{head}} · index {{index}}',
  'provenance.lineNoRun': '{{source}} · HEAD {{head}} · index {{index}}',
  'provenance.indexBehind': 'The code index is at a different commit than HEAD',
  'provenance.unknownValue': 'unknown',
  'stale.title': 'This result is out of date',
  'stale.headMoved': 'HEAD has moved since this result was computed.',
  'stale.backend': 'The backend marked this result as stale.',
  'stale.cache': 'The repository changed after this result was fetched; it is being refreshed.',
  'stale.rerun': 'Run again',
  'stale.label': 'stale',
  'banner.dirty':
    'The working tree had uncommitted changes during the run; line numbers may be off.',
  'banner.changedDuringRun': 'The working tree changed during the run; line numbers may be off.',
  'banner.scopeWidened': 'The scope was widened compared with the request.',
  'banner.outsideScope':
    '{{count}} findings are outside the changed scope and are not counted in the gate',
  'banner.truncated': 'The backend truncated the findings of this run.',
  'counts.title': 'Findings by severity',
  'counts.description': 'Counts of errors, warnings and info from the runs behind this verdict.',
  // Local versus CI
  'ci.title': 'Local versus CI',
  'ci.agree_pass': 'Local and CI both passed',
  'ci.agree_fail': 'Local and CI both failed',
  'ci.local_pass_ci_fail': 'Local passed but CI did not; this cannot be treated as passing',
  'ci.local_fail_ci_pass': 'Local failed but CI passed',
  'ci.local_only': 'Only a local run exists',
  'ci.ci_only': 'Only a CI run exists',
  'ci.ci_pending': 'CI is still running',
  'ci.sha_mismatch': 'CI ran on a different commit than HEAD',
  'ci.not_comparable': 'Local and CI results cannot be compared',
  'ci.unknown': 'The comparison is not known',
  'ci.open': 'Open CI run',
  'ci.hints': 'Possible causes',
  // Chip
  'chip.label': 'Quality',
  'chip.open': 'Open the quality view',
  // Run control
  'run.profile': 'Profile',
  'run.scopeLabel': 'Scope',
  'run.scope.changed': 'Changed files',
  'run.scope.worktree': 'Whole worktree',
  'run.scope.commitRange': 'Commit range',
  'run.start': 'Run checks',
  'run.starting': 'Starting',
  'run.heavy': 'heavy',
  'run.notReady': 'Not ready',
  'run.missing': 'Missing: {{check}} ({{reason}})',
  'run.missingHint': 'Hint: {{hint}}',
  'run.noProfiles': 'No runnable profiles are available',
  'run.cancel': 'Cancel',
  'run.cancelling': 'Cancelling, waiting for confirmation',
  'run.queued': 'Queued',
  'run.running': 'Running',
  'run.stage': 'Stage: {{stage}}',
  'run.step': 'Step {{index}} of {{count}}',
  'run.percent': '{{percent}}%',
  'run.progressUnknown': 'Progress is not known',
  'run.finished.succeeded': 'The run finished',
  'run.finished.failed': 'The run did not complete',
  'run.finished.interrupted': 'The run was interrupted and did not complete',
  'run.finished.cancelled': 'The run was cancelled',
  'run.finished.unknown': 'The run ended in an unknown state',
  'run.dismiss': 'Dismiss',
  'run.copyDetails': 'Copy details',
  'run.details': 'Run {{runId}} · {{status}}',
  // Start errors
  'error.envNotReady': 'The environment is not ready to run these checks',
  'error.recheck': 'Check again',
  'error.profileUnknown': 'That profile is no longer available; the list was refreshed',
  'error.profileAvailable': 'Available: {{names}}',
  'error.rateLimited': 'Too many runs were requested',
  'error.rateLimitedIn': 'Try again in {{seconds}} s',
  'error.forbidden': 'You do not have permission to run checks; results stay readable',
  'error.offline': 'You are offline: cached results are shown and running is unavailable',
  'error.generic': 'The checks could not be started',
  // State screens
  'state.loading': 'Loading quality results',
  'state.notRun.title': 'The checks have not run for this scope',
  'state.notRun.body': 'Run the checks to get a verdict. Until then nothing is concluded.',
  'state.empty': 'No findings in the checks that ran ({{profile}})',
  'state.error': 'Quality results could not be loaded',
  'state.retry': 'Retry',
  'state.forbidden': 'You do not have access to quality results',
  'state.offline': 'You are offline: showing the last cached result',
  'state.indexStale': 'The code index is behind HEAD; structural checks may miss recent changes',
  'state.runFailed': 'The last run did not complete, so no verdict can be drawn from it',
  'state.cancelled': 'The last run was cancelled'
} as const

export type QualityScorecardCopyKey = keyof typeof QUALITY_SCORECARD_COPY

export const QUALITY_SCORECARD_COPY_GROUP = 'scorecard'

export const scorecardCopy = createQualityCopy(QUALITY_SCORECARD_COPY_GROUP, QUALITY_SCORECARD_COPY)
