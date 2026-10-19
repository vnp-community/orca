// Task-related types cho KG context và quality gate (CR-ORCA-002, CR-ORCA-003)

// ─── KG Context (CR-ORCA-002) ─────────────────────────────────────────────────

// Mirror của Go TaskKGContext struct (SOL-ORCA-002 §2.2)
// snake_case để match JSON API từ Go json:"field_name" tags
export interface TaskKGContext {
  crs_id?: string // Go: CRSID "CRS-045"
  feature_ref?: string // Go: FeatureRef "F27"
  service?: string // Go: Service "ai-service"
  repo_target?: string // Go: RepoTarget "vnp-workspace" (dùng cho convention verify)
}

// EnrichedTask enrichment cache per-task (mirror SOL-ORCA-002 §2.5)
// crsContext và featureContext là raw JSON (json.RawMessage equivalent)
export interface TaskEnrichment {
  taskId: string
  crsContext?: unknown // pis_query_crs result
  featureContext?: unknown // pis_query_feature_context result
  fetchedAt: number // epoch ms — cache TTL 5 phút
  loading: boolean
  error?: string
}

// ─── Task Rejection (CR-ORCA-003) ─────────────────────────────────────────────

// Mirror JSON của TaskRejectedResponse struct (SOL-ORCA-003 §2.5)
// field names khớp JSON: task_id, status, reason, action_required
export interface TaskRejection {
  task_id: string // backend: TaskID
  status: 'rejected' // always "rejected"
  reason: string // "convention check failed: 1 files missing '// implements: CRS-045', 0 invalid"
  action_required: string // "Add '// implements: CRS-045' comment to each changed file"
}

// TaskStatus — mở rộng với 'rejected' (SOL-ORCA-003 quality gate)
export type TaskStatus =
  | 'pending'
  | 'in-progress'
  | 'done'
  | 'rejected' // quality gate fail từ CompletionHook
  | 'cancelled'
