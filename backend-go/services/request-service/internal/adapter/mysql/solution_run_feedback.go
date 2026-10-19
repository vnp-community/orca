package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunFeedbackByIDs reads the feedback of analysis runs: one IN query per batch, scoped to the tenant.
func (r *SolutionRecordRepository) RunFeedbackByIDs(ctx context.Context, runIDs []string) (map[string]string, error) {
	out := map[string]string{}
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		for _, batch := range usecase.RunFeedbackBatches(runIDs) {
			args := make([]any, 0, len(batch)+1)
			args = append(args, tenantID)
			for _, id := range batch {
				args = append(args, id)
			}
			rows, err := db.QueryContext(ctx, `SELECT id, feedback FROM analysis_runs
				WHERE tenant_id = ? AND id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")+`) AND feedback IS NOT NULL AND feedback <> ''`, args...)
			if err != nil {
				return fmt.Errorf("mysql: run feedback: %w", err)
			}
			for rows.Next() {
				var id, feedback string
				if err := rows.Scan(&id, &feedback); err != nil {
					rows.Close()
					return fmt.Errorf("mysql: scan run feedback: %w", err)
				}
				out[id] = feedback
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("mysql: run feedback rows: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
