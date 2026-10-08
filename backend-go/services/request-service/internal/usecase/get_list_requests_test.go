package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type recordingRequestRepository struct {
	RequestRepository
	getCalls  int
	gotFilter ListFilter
	getErr    error
}

func (r *recordingRequestRepository) Get(_ context.Context, id string) (domain.Request, error) {
	r.getCalls++
	return domain.Request{ID: id}, r.getErr
}

func (r *recordingRequestRepository) List(_ context.Context, f ListFilter) (ListResult, error) {
	r.gotFilter = f
	return ListResult{NextPageToken: "next"}, nil
}

func codeOf(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func tenantContext() context.Context { return tenant.WithTenantID(context.Background(), "t1") }

func TestGetRequest_MalformedIDIsNotFoundWithoutTouchingRepo(t *testing.T) {
	repo := &recordingRequestRepository{}
	_, err := NewGetRequest(repo).Execute(tenantContext(), "not-a-uuid")
	if codeOf(err) != "REQUEST_NOT_FOUND" || repo.getCalls != 0 {
		t.Fatalf("err=%v calls=%d, want REQUEST_NOT_FOUND and no repo call", err, repo.getCalls)
	}
}

func TestGetRequest_RequiresTenant(t *testing.T) {
	_, err := NewGetRequest(&recordingRequestRepository{}).Execute(context.Background(), uuid.NewString())
	if codeOf(err) != "REQUEST_TENANT_REQUIRED" {
		t.Fatalf("err = %v", err)
	}
}

func TestGetRequest_PassesRepoErrorThrough(t *testing.T) {
	want := domain.ErrRequestNotFound("x")
	_, err := NewGetRequest(&recordingRequestRepository{getErr: want}).Execute(tenantContext(), uuid.NewString())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestListRequests_NormalizesFilterBeforeRepo(t *testing.T) {
	repo := &recordingRequestRepository{}
	res, err := NewListRequests(repo).Execute(tenantContext(), ListFilter{PageSize: 900})
	if err != nil || res.NextPageToken != "next" || repo.gotFilter.PageSize != 200 {
		t.Fatalf("res=%+v err=%v filter=%+v", res, err, repo.gotFilter)
	}
	if _, err := NewListRequests(repo).Execute(tenantContext(), ListFilter{PageSize: -1}); codeOf(err) != "INVALID_PAGE_SIZE" {
		t.Fatalf("negative page size: %v", err)
	}
	if _, err := NewListRequests(repo).Execute(context.Background(), ListFilter{}); codeOf(err) != "REQUEST_TENANT_REQUIRED" {
		t.Fatalf("missing tenant: %v", err)
	}
}
