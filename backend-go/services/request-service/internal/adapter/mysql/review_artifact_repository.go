package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ReviewArtifactRepository stores the drawings of reviews and their content.
type ReviewArtifactRepository struct {
	*Repository
}

func NewReviewArtifactRepository(r *Repository) *ReviewArtifactRepository {
	return &ReviewArtifactRepository{Repository: r}
}

var _ usecase.ReviewArtifactRepository = (*ReviewArtifactRepository)(nil)

const artifactColumns = `id, request_id, section, variant, name, diagram_type, input_digest, preset_digest, skill_version, revision,
	state, origin, reason, error_code, blob_digest, size_bytes, view_box, title, description, self_check, model, COALESCE(created_by, ''), created_at`

func scanArtifact(row interface{ Scan(dest ...any) error }, tenantID string) (domain.ReviewArtifact, error) {
	var a domain.ReviewArtifact
	var section, dtype, state, origin string
	if err := row.Scan(&a.ID, &a.RequestID, &section, &a.Key.Variant, &a.Key.Name, &dtype, &a.Key.InputDigest, &a.Key.PresetDigest, &a.Key.SkillVersion,
		&a.Revision, &state, &origin, &a.Reason, &a.ErrorCode, &a.BlobDigest, &a.SizeBytes, &a.ViewBox, &a.Title, &a.Desc, &a.SelfCheck, &a.Model,
		&a.CreatedBy, &a.CreatedAt); err != nil {
		return domain.ReviewArtifact{}, err
	}
	a.TenantID, a.Key.Section, a.Type = tenantID, domain.ReviewSection(section), domain.DiagramType(dtype)
	a.State, a.Origin, a.CreatedAt = domain.ReviewArtifactState(state), domain.ReviewArtifactOrigin(origin), a.CreatedAt.UTC()
	return a, nil
}

func slotKey(tenantID, requestID string, k domain.ArtifactKey) string {
	return tenantID + ":" + requestID + ":" + k.Slot()
}

const maxRevisionRetries = 8

// retryable is what two writers of one slot can do to each other: a duplicate revision or active key, or a deadlock between their
// locks. The loser starts over and reads the winner's rows.
func retryable(err error) bool {
	var myErr *driver.MySQLError
	if errors.As(err, &myErr) && (myErr.Number == 1213 || myErr.Number == 1205) {
		return true
	}
	return isDuplicateKey(err, "uq_review_artifacts_revision") || isDuplicateKey(err, "uq_review_artifacts_active")
}

func (r *ReviewArtifactRepository) Save(ctx context.Context, in domain.ReviewArtifact, blob *domain.ReviewBlob) (domain.ReviewArtifact, error) {
	if err := in.Key.Valid(); err != nil {
		return domain.ReviewArtifact{}, domain.ErrReviewArtifactUnsafe(err.Error())
	}
	var out domain.ReviewArtifact
	var err error
	for attempt := 0; attempt < maxRevisionRetries; attempt++ {
		err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
			if blob != nil {
				if _, err := db.ExecContext(ctx, `INSERT INTO review_artifact_blobs (tenant_id, request_id, digest, media_type, size_bytes, content)
					VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE created_at = CURRENT_TIMESTAMP(6)`, tenantID, in.RequestID, blob.Digest, blob.MediaType, len(blob.Content), string(blob.Content)); err != nil {
					return fmt.Errorf("mysql: store review blob: %w", err)
				}
			}
			var rev int
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision), 0) + 1 FROM review_artifacts
				WHERE tenant_id = ? AND request_id = ? AND section = ? AND variant = ? AND name = ? AND input_digest = ? AND preset_digest = ? AND skill_version = ?`,
				tenantID, in.RequestID, string(in.Key.Section), in.Key.Variant, in.Key.Name, in.Key.InputDigest, in.Key.PresetDigest, in.Key.SkillVersion).Scan(&rev); err != nil {
				return err
			}
			var activeKey any
			if in.State == domain.ArtifactActive {
				activeKey = slotKey(tenantID, in.RequestID, in.Key)
				if _, err := db.ExecContext(ctx, `UPDATE review_artifacts SET state = 'superseded', active_key = NULL WHERE tenant_id = ? AND active_key = ?`, tenantID, activeKey); err != nil {
					return err
				}
			}
			model := in.Model
			if len(model) == 0 {
				model = []byte(`{}`)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO review_artifacts (id, tenant_id, request_id, section, variant, name, diagram_type, input_digest, preset_digest,
				skill_version, revision, state, origin, reason, error_code, blob_digest, size_bytes, view_box, title, description, self_check, model, created_by, active_key)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				in.ID, tenantID, in.RequestID, string(in.Key.Section), in.Key.Variant, in.Key.Name, string(in.Type), in.Key.InputDigest, in.Key.PresetDigest,
				in.Key.SkillVersion, rev, string(in.State), string(in.Origin), in.Reason, in.ErrorCode, in.BlobDigest, in.SizeBytes, in.ViewBox, in.Title, in.Desc,
				in.SelfCheck, string(model), nullIfEmpty(in.CreatedBy), activeKey); err != nil {
				return err
			}
			var serr error
			out, serr = scanArtifact(db.QueryRowContext(ctx, `SELECT `+artifactColumns+` FROM review_artifacts WHERE tenant_id = ? AND id = ?`, tenantID, in.ID), tenantID)
			return serr
		})
		if err == nil || !retryable(err) {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 3 * time.Millisecond):
		}
	}
	return out, err
}

func (r *ReviewArtifactRepository) one(ctx context.Context, where string, args ...any) (domain.ReviewArtifact, bool, error) {
	var out domain.ReviewArtifact
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		out, err = scanArtifact(db.QueryRowContext(ctx, `SELECT `+artifactColumns+` FROM review_artifacts WHERE tenant_id = ? AND `+where, append([]any{tenantID}, args...)...), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *ReviewArtifactRepository) FindByKey(ctx context.Context, requestID string, k domain.ArtifactKey) (domain.ReviewArtifact, bool, error) {
	if notUUID(requestID) {
		return domain.ReviewArtifact{}, false, nil
	}
	return r.one(ctx, `request_id = ? AND section = ? AND variant = ? AND name = ? AND input_digest = ? AND preset_digest = ? AND skill_version = ?
		ORDER BY revision DESC LIMIT 1`, requestID, string(k.Section), k.Variant, k.Name, k.InputDigest, k.PresetDigest, k.SkillVersion)
}

func (r *ReviewArtifactRepository) Active(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) (domain.ReviewArtifact, bool, error) {
	if notUUID(requestID) {
		return domain.ReviewArtifact{}, false, nil
	}
	return r.one(ctx, `request_id = ? AND section = ? AND variant = ? AND name = ? AND state = 'active'`, requestID, string(section), variant, name)
}

func (r *ReviewArtifactRepository) Get(ctx context.Context, requestID, id string) (domain.ReviewArtifact, error) {
	if notUUID(requestID) || notUUID(id) {
		return domain.ReviewArtifact{}, domain.ErrReviewArtifactNotFound(id)
	}
	a, ok, err := r.one(ctx, `request_id = ? AND id = ?`, requestID, id)
	if err != nil {
		return domain.ReviewArtifact{}, err
	}
	if !ok {
		return domain.ReviewArtifact{}, domain.ErrReviewArtifactNotFound(id)
	}
	return a, nil
}

func (r *ReviewArtifactRepository) Activate(ctx context.Context, requestID, id string) (domain.ReviewArtifact, error) {
	var out domain.ReviewArtifact
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) || notUUID(id) {
			return domain.ErrReviewArtifactNotFound(id)
		}
		cur, err := scanArtifact(db.QueryRowContext(ctx, `SELECT `+artifactColumns+` FROM review_artifacts WHERE tenant_id = ? AND request_id = ? AND id = ? FOR UPDATE`,
			tenantID, requestID, id), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrReviewArtifactNotFound(id)
		}
		if err != nil {
			return err
		}
		if cur.State == domain.ArtifactFailed || cur.BlobDigest == "" {
			return domain.ErrReviewArtifactUnsafe("a failed drawing cannot be restored")
		}
		key := slotKey(tenantID, requestID, cur.Key)
		if _, err := db.ExecContext(ctx, `UPDATE review_artifacts SET state = 'superseded', active_key = NULL WHERE tenant_id = ? AND active_key = ? AND id <> ?`, tenantID, key, id); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE review_artifacts SET state = 'active', active_key = ? WHERE tenant_id = ? AND request_id = ? AND id = ?`, key, tenantID, requestID, id); err != nil {
			return err
		}
		out, err = scanArtifact(db.QueryRowContext(ctx, `SELECT `+artifactColumns+` FROM review_artifacts WHERE tenant_id = ? AND id = ?`, tenantID, id), tenantID)
		return err
	})
	return out, err
}

func (r *ReviewArtifactRepository) list(ctx context.Context, where string, args ...any) ([]domain.ReviewArtifact, error) {
	var out []domain.ReviewArtifact
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+artifactColumns+` FROM review_artifacts WHERE tenant_id = ? AND `+where, append([]any{tenantID}, args...)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a, err := scanArtifact(rows, tenantID)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func (r *ReviewArtifactRepository) ListRevisions(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) ([]domain.ReviewArtifact, error) {
	if notUUID(requestID) {
		return nil, nil
	}
	return r.list(ctx, `request_id = ? AND section = ? AND variant = ? AND name = ? ORDER BY created_at DESC, id DESC LIMIT 100`, requestID, string(section), variant, name)
}

func (r *ReviewArtifactRepository) ListActive(ctx context.Context, requestID string) ([]domain.ReviewArtifact, error) {
	if notUUID(requestID) {
		return nil, nil
	}
	return r.list(ctx, `request_id = ? AND state = 'active' ORDER BY section, variant, name`, requestID)
}

func (r *ReviewArtifactRepository) Blob(ctx context.Context, requestID, digest string) (domain.ReviewBlob, bool, error) {
	var out domain.ReviewBlob
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var content string
		err := db.QueryRowContext(ctx, `SELECT digest, media_type, content, created_at FROM review_artifact_blobs WHERE tenant_id = ? AND request_id = ? AND digest = ?`,
			tenantID, requestID, digest).Scan(&out.Digest, &out.MediaType, &content, &out.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		out.Content, out.CreatedAt, found = []byte(content), out.CreatedAt.UTC(), err == nil
		return err
	})
	return out, found, err
}

func (r *ReviewArtifactRepository) StoredBytes(ctx context.Context) (int64, error) {
	var n int64
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM review_artifact_blobs WHERE tenant_id = ?`, tenantID).Scan(&n)
	})
	return n, err
}

func (r *ReviewArtifactRepository) DeleteSupersededBefore(ctx context.Context, before time.Time, limit int) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `DELETE FROM review_artifacts WHERE tenant_id = ? AND state <> 'active' AND created_at < ? ORDER BY created_at LIMIT ?`, tenantID, before.UTC(), limit)
		if err != nil {
			return err
		}
		c, _ := res.RowsAffected()
		n = int(c)
		return nil
	})
	return n, err
}

func (r *ReviewArtifactRepository) DeleteOrphanBlobs(ctx context.Context, olderThan time.Time, limit int) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `DELETE FROM review_artifact_blobs WHERE tenant_id = ? AND created_at < ?
			AND NOT EXISTS (SELECT 1 FROM review_artifacts a WHERE a.tenant_id = review_artifact_blobs.tenant_id AND a.request_id = review_artifact_blobs.request_id AND a.blob_digest = review_artifact_blobs.digest) LIMIT ?`,
			tenantID, olderThan.UTC(), limit)
		if err != nil {
			return err
		}
		c, _ := res.RowsAffected()
		n = int(c)
		return nil
	})
	return n, err
}
