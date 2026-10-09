//go:build integration

// TASK-E2E-017: an unknown annotation id answers ErrAnnotationNotFound on every dialect.
package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

func TestRepository_UnknownID_IsNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, id := range []string{uuid.NewString(), "abc"} {
		_, err := repo.GetAnnotation(ctx, tenantID, id)
		if !errors.Is(err, domain.ErrAnnotationNotFound) {
			t.Errorf("GetAnnotation(%q): want ErrAnnotationNotFound, got %v", id, err)
		}
		_, err = repo.UpdateAnnotation(ctx, tenantID, id, "x", false)
		if !errors.Is(err, domain.ErrAnnotationNotFound) {
			t.Errorf("UpdateAnnotation(%q): want ErrAnnotationNotFound, got %v", id, err)
		}
		if err := repo.DeleteAnnotation(ctx, tenantID, id); !errors.Is(err, domain.ErrAnnotationNotFound) {
			t.Errorf("DeleteAnnotation(%q): want ErrAnnotationNotFound, got %v", id, err)
		}
	}

	out, err := repo.MarkSent(ctx, tenantID, []string{"abc", uuid.NewString()}, time.Now())
	if err != nil || len(out) != 0 {
		t.Errorf("MarkSent(malformed+unknown) = %v, %v; want empty, nil", out, err)
	}
}
