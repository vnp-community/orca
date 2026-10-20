package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

const artifactColumns = `id::text, request_id::text, section, variant, name, diagram_type, input_digest, preset_digest, skill_version, revision,
	state, origin, reason, error_code, blob_digest, size_bytes, view_box, title, description, self_check, model, COALESCE(created_by::text, ''), created_at`

func scanArtifact(row pgx.Row, tenantID string) (domain.ReviewArtifact, error) {
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

const maxRevisionRetries = 3

// lockSlot makes writers of one slot take turns until the transaction ends. It touches no table; the key begins with the tenant id.
func lockSlot(ctx context.Context, db dbExecer, key string) error {
	_, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key)
	return err
}

func (r *ReviewArtifactRepository) Save(ctx context.Context, in domain.ReviewArtifact, blob *domain.ReviewBlob) (domain.ReviewArtifact, error) {
	if err := in.Key.Valid(); err != nil {
		return domain.ReviewArtifact{}, domain.ErrReviewArtifactUnsafe(err.Error())
	}
	var out domain.ReviewArtifact
	var err error
	for attempt := 0; attempt < maxRevisionRetries; attempt++ {
		err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
			// A blob that is already there is touched, not skipped: the cleanup deletes only blobs untouched for a while, and the touch is
			// what makes it wait for this transaction instead of deleting the content a new row is about to point at.
			// Writers of one slot take turns: each reads the newest revision and the active drawing, and both must still be true at insert.
			if err := lockSlot(ctx, db, slotKey(tenantID, in.RequestID, in.Key)); err != nil {
				return err
			}
			if blob != nil {
				if _, err := db.Exec(ctx, `INSERT INTO request.review_artifact_blobs (tenant_id, request_id, digest, media_type, size_bytes, content)
					VALUES ($1, $2, $3, $4, $5, $6)
					ON CONFLICT (tenant_id, request_id, digest) DO UPDATE SET created_at = clock_timestamp()`,
					tenantID, in.RequestID, blob.Digest, blob.MediaType, len(blob.Content), string(blob.Content)); err != nil {
					return fmt.Errorf("postgres: store review blob: %w", err)
				}
			}
			var rev int
			if err := db.QueryRow(ctx, `SELECT COALESCE(MAX(revision), 0) + 1 FROM request.review_artifacts
				WHERE tenant_id = $1 AND request_id = $2 AND section = $3 AND variant = $4 AND name = $5 AND input_digest = $6 AND preset_digest = $7 AND skill_version = $8`,
				tenantID, in.RequestID, string(in.Key.Section), in.Key.Variant, in.Key.Name, in.Key.InputDigest, in.Key.PresetDigest, in.Key.SkillVersion).Scan(&rev); err != nil {
				return err
			}
			var activeKey any
			if in.State == domain.ArtifactActive {
				activeKey = slotKey(tenantID, in.RequestID, in.Key)
				if _, err := db.Exec(ctx, `UPDATE request.review_artifacts SET state = 'superseded', active_key = NULL
					WHERE tenant_id = $1 AND active_key = $2`, tenantID, activeKey); err != nil {
					return err
				}
			}
			model := in.Model
			if len(model) == 0 {
				model = []byte(`{}`)
			}
			row := db.QueryRow(ctx, `INSERT INTO request.review_artifacts (id, tenant_id, request_id, section, variant, name, diagram_type, input_digest, preset_digest,
				skill_version, revision, state, origin, reason, error_code, blob_digest, size_bytes, view_box, title, description, self_check, model, created_by, active_key)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22::jsonb,$23,$24) RETURNING `+artifactColumns,
				in.ID, tenantID, in.RequestID, string(in.Key.Section), in.Key.Variant, in.Key.Name, string(in.Type), in.Key.InputDigest, in.Key.PresetDigest,
				in.Key.SkillVersion, rev, string(in.State), string(in.Origin), in.Reason, in.ErrorCode, in.BlobDigest, in.SizeBytes, in.ViewBox, in.Title, in.Desc,
				in.SelfCheck, string(model), nullIfEmpty(in.CreatedBy), activeKey)
			var serr error
			out, serr = scanArtifact(row, tenantID)
			return serr
		})
		if err == nil || !(isUniqueViolation(err, "uq_review_artifacts_revision") || isUniqueViolation(err, "uq_review_artifacts_active")) {
			return out, err
		}
	}
	return out, err
}

func (r *ReviewArtifactRepository) one(ctx context.Context, where string, args ...any) (domain.ReviewArtifact, bool, error) {
	var out domain.ReviewArtifact
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		out, err = scanArtifact(db.QueryRow(ctx, `SELECT `+artifactColumns+` FROM request.review_artifacts WHERE tenant_id = $1 AND `+where, append([]any{tenantID}, args...)...), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
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
	return r.one(ctx, `request_id = $2 AND section = $3 AND variant = $4 AND name = $5 AND input_digest = $6 AND preset_digest = $7 AND skill_version = $8
		ORDER BY revision DESC LIMIT 1`, requestID, string(k.Section), k.Variant, k.Name, k.InputDigest, k.PresetDigest, k.SkillVersion)
}

func (r *ReviewArtifactRepository) Active(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) (domain.ReviewArtifact, bool, error) {
	if notUUID(requestID) {
		return domain.ReviewArtifact{}, false, nil
	}
	return r.one(ctx, `request_id = $2 AND section = $3 AND variant = $4 AND name = $5 AND state = 'active'`, requestID, string(section), variant, name)
}

func (r *ReviewArtifactRepository) Get(ctx context.Context, requestID, id string) (domain.ReviewArtifact, error) {
	if notUUID(requestID) || notUUID(id) {
		return domain.ReviewArtifact{}, domain.ErrReviewArtifactNotFound(id)
	}
	a, ok, err := r.one(ctx, `request_id = $2 AND id = $3`, requestID, id)
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
		var section, variant, name string
		if err := db.QueryRow(ctx, `SELECT section, variant, name FROM request.review_artifacts WHERE tenant_id = $1 AND request_id = $2 AND id = $3`, tenantID, requestID, id).Scan(&section, &variant, &name); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrReviewArtifactNotFound(id)
			}
			return err
		}
		if err := lockSlot(ctx, db, slotKey(tenantID, requestID, domain.ArtifactKey{Section: domain.ReviewSection(section), Variant: variant, Name: name})); err != nil {
			return err
		}
		cur, err := scanArtifact(db.QueryRow(ctx, `SELECT `+artifactColumns+` FROM request.review_artifacts WHERE tenant_id = $1 AND request_id = $2 AND id = $3 FOR UPDATE`,
			tenantID, requestID, id), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrReviewArtifactNotFound(id)
		}
		if err != nil {
			return err
		}
		if cur.State == domain.ArtifactFailed || cur.BlobDigest == "" {
			return domain.ErrReviewArtifactUnsafe("a failed drawing cannot be restored")
		}
		key := slotKey(tenantID, requestID, cur.Key)
		if _, err := db.Exec(ctx, `UPDATE request.review_artifacts SET state = 'superseded', active_key = NULL WHERE tenant_id = $1 AND active_key = $2 AND id <> $3`, tenantID, key, id); err != nil {
			return err
		}
		out, err = scanArtifact(db.QueryRow(ctx, `UPDATE request.review_artifacts SET state = 'active', active_key = $4 WHERE tenant_id = $1 AND request_id = $2 AND id = $3 RETURNING `+artifactColumns,
			tenantID, requestID, id, key), tenantID)
		return err
	})
	return out, err
}

func (r *ReviewArtifactRepository) list(ctx context.Context, where string, args ...any) ([]domain.ReviewArtifact, error) {
	var out []domain.ReviewArtifact
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT `+artifactColumns+` FROM request.review_artifacts WHERE tenant_id = $1 AND `+where, append([]any{tenantID}, args...)...)
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
	return r.list(ctx, `request_id = $2 AND section = $3 AND variant = $4 AND name = $5 ORDER BY created_at DESC, id DESC LIMIT 100`, requestID, string(section), variant, name)
}

func (r *ReviewArtifactRepository) ListActive(ctx context.Context, requestID string) ([]domain.ReviewArtifact, error) {
	if notUUID(requestID) {
		return nil, nil
	}
	return r.list(ctx, `request_id = $2 AND state = 'active' ORDER BY section, variant, name`, requestID)
}

func (r *ReviewArtifactRepository) Blob(ctx context.Context, requestID, digest string) (domain.ReviewBlob, bool, error) {
	var out domain.ReviewBlob
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var content string
		err := db.QueryRow(ctx, `SELECT digest, media_type, content, created_at FROM request.review_artifact_blobs WHERE tenant_id = $1 AND request_id = $2 AND digest = $3`,
			tenantID, requestID, digest).Scan(&out.Digest, &out.MediaType, &content, &out.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
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
		return db.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM request.review_artifact_blobs WHERE tenant_id = $1`, tenantID).Scan(&n)
	})
	return n, err
}

func (r *ReviewArtifactRepository) DeleteSupersededBefore(ctx context.Context, before time.Time, limit int) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `DELETE FROM request.review_artifacts WHERE tenant_id = $1 AND id IN (
			SELECT id FROM request.review_artifacts WHERE tenant_id = $1 AND state <> 'active' AND created_at < $2 ORDER BY created_at LIMIT $3)`, tenantID, before, limit)
		n = int(tag.RowsAffected())
		return err
	})
	return n, err
}

func (r *ReviewArtifactRepository) DeleteOrphanBlobs(ctx context.Context, olderThan time.Time, limit int) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `DELETE FROM request.review_artifact_blobs b WHERE b.tenant_id = $1 AND (b.request_id, b.digest) IN (
			SELECT x.request_id, x.digest FROM request.review_artifact_blobs x
			WHERE x.tenant_id = $1 AND x.created_at < $2
			AND NOT EXISTS (SELECT 1 FROM request.review_artifacts a WHERE a.tenant_id = x.tenant_id AND a.request_id = x.request_id AND a.blob_digest = x.digest)
			LIMIT $3)`, tenantID, olderThan, limit)
		n = int(tag.RowsAffected())
		return err
	})
	return n, err
}
