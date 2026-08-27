// Why: `GitHubItemDialog.tsx` and `PullRequestPage.tsx` duplicate this type
// verbatim (PullRequestPage.tsx's header comment calls itself "duplicated
// from GitHubItemDialog"). Extracted per TASK-BIGFILE-017 so both files
// import one definition instead of drifting copies.
//
// Note: `invalidateWorkItemDetailsCacheForKey` was in-scope for the same
// task but was NOT moved here. It reads/writes module-private mutable state
// (`workItemDetailsCache` Map, `workItemDetailsCacheGeneration` counter,
// `notifyWorkItemDetailsCache` listeners) that each source file defines and
// uses independently across ~10 sibling symbols and ~15 call sites spanning
// the whole SWR-style details cache subsystem in each file. Moving only the
// invalidate function here would either silently merge the two files' caches
// into one (a behavior change) or force a signature change (no longer a pure
// copy). See TASK-BIGFILE-017 status notes / SOLUTION-FE-BIGFILE-005 for
// detail.
export type ItemDialogTab = 'conversation' | 'checks' | 'files'
