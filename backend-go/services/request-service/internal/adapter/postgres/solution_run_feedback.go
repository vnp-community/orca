package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunFeedbackByIDs reads the feedback of analysis runs: one query per batch, scoped to the tenant. Ids that are not UUIDs
// cannot name a run, so they are skipped instead of failing the cast for the whole batch.
func (r *SolutionRecordRepository) RunFeedbackByIDs(ctx context.Context, runIDs []string) (map[string]string, error) {
	out := map[string]string{}
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		for _, batch := range usecase.RunFeedbackBatches(runIDs) {
			valid := batch[:0:0]
			for _, id := range batch {
				if _, perr := uuid.Parse(id); perr == nil {
					valid = append(valid, id)
				}
			}
			if len(valid) == 0 {
				continue
			}
			rows, err := db.Query(ctx, `SELECT id::text, feedback FROM request.analysis_runs
				WHERE tenant_id = $1 AND id = ANY($2::uuid[]) AND feedback IS NOT NULL AND feedback <> ''`, tenantID, valid)
			if err != nil {
				return fmt.Errorf("postgres: run feedback: %w", err)
			}
			for rows.Next() {
				var id, feedback string
				if err := rows.Scan(&id, &feedback); err != nil {
					rows.Close()
					return fmt.Errorf("postgres: scan run feedback: %w", err)
				}
				out[id] = feedback
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("postgres: run feedback rows: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
