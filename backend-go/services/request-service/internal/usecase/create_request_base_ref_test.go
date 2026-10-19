package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func baseRefInput(ref string) CreateRequestInput {
	in := manualInput("needs a branch")
	in.BaseRef = ref
	return in
}

func TestCreateRequest_BaseRef_Empty(t *testing.T) {
	f := newCreateFixture()
	checker := &fakeRefChecker{}
	f.uc.WithBaseRefChecker(checker)
	res, err := f.uc.Execute(tctx(), baseRefInput(""))
	if err != nil || res.Request.BaseRef != "" || res.BaseRefUnverified || checker.calls != 0 {
		t.Fatalf("%+v %v checker calls=%d", res, err, checker.calls)
	}
}

func TestCreateRequest_BaseRef_EmptyNeedsNoChecker(t *testing.T) {
	f := newCreateFixture() // no checker at all
	if res, err := f.uc.Execute(tctx(), baseRefInput("")); err != nil || res.Request.BaseRef != "" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestCreateRequest_BaseRef_Valid(t *testing.T) {
	f := newCreateFixture()
	checker := &fakeRefChecker{}
	f.uc.WithBaseRefChecker(checker)
	res, err := f.uc.Execute(tctx(), baseRefInput(" release/1.4 "))
	if err != nil || res.Request.BaseRef != "release/1.4" || res.BaseRefUnverified {
		t.Fatalf("%+v %v", res, err)
	}
	if checker.calls != 1 || checker.ref != "release/1.4" || checker.project != testProject {
		t.Fatalf("checker saw %q for %q", checker.ref, checker.project)
	}
	if stored := f.s.requests[res.Request.ID]; stored.BaseRef != "release/1.4" {
		t.Fatalf("not stored: %+v", stored)
	}
}

func TestCreateRequest_BaseRef_Invalid(t *testing.T) {
	f := newCreateFixture()
	checker := &fakeRefChecker{}
	f.uc.WithBaseRefChecker(checker)
	for _, bad := range []string{"-x", "a..b", "   ", "a~1", strings.Repeat("a", 256)} {
		_, err := f.uc.Execute(tctx(), baseRefInput(bad))
		mustCode(t, err, "REQUEST_BASE_REF_INVALID")
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Kind != apperrors.KindInvalidArgument {
			t.Fatalf("kind: %v", err)
		}
	}
	if checker.calls != 0 || len(f.s.requests) != 0 {
		t.Fatalf("an invalid ref must reach neither git nor the store: calls=%d stored=%d", checker.calls, len(f.s.requests))
	}
}

func TestCreateRequest_BaseRef_NotFound(t *testing.T) {
	f := newCreateFixture()
	f.uc.WithBaseRefChecker(&fakeRefChecker{check: BaseRefCheck{Status: BaseRefNotFound, Suggestions: []string{"main", "release/1.3"}}})
	_, err := f.uc.Execute(tctx(), baseRefInput("release/9"))
	mustCode(t, err, "REQUEST_BASE_REF_NOT_FOUND")
	if d, _ := domain.BaseRefDetails(err); d["suggestions"] != "main,release/1.3" {
		t.Fatalf("suggestions: %v", d)
	}
	if len(f.s.requests) != 0 || f.s.counter != 0 {
		t.Fatalf("nothing may be created and no number burnt: %d requests, counter %d", len(f.s.requests), f.s.counter)
	}
}

func TestCreateRequest_BaseRef_Unreachable(t *testing.T) {
	for name, checker := range map[string]*fakeRefChecker{
		"unreachable": {check: BaseRefCheck{Status: BaseRefUnreachable}},
		"error":       {err: errors.New("agent offline")},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCreateFixture()
			f.uc.WithBaseRefChecker(checker)
			res, err := f.uc.Execute(tctx(), baseRefInput("release/1.4"))
			if err != nil || !res.Created || !res.BaseRefUnverified || res.Request.BaseRef != "release/1.4" {
				t.Fatalf("fail-open: %+v %v", res, err)
			}
		})
	}
}

func TestCreateRequest_BaseRef_NoCheckerConfiguredRefusesARef(t *testing.T) {
	f := newCreateFixture()
	_, err := f.uc.Execute(tctx(), baseRefInput("release/1.4"))
	mustCode(t, err, "REQUEST_BASE_REF_CHECK_NOT_CONFIGURED")
	if len(f.s.requests) != 0 {
		t.Fatal("a ref must never be accepted unchecked because of a wiring gap")
	}
}

func TestCreateRequest_BaseRef_IdempotentReplayKeepsTheFirstRef(t *testing.T) {
	f := newCreateFixture()
	f.uc.WithBaseRefChecker(&fakeRefChecker{})
	in := baseRefInput("release/1.4")
	in.ClientRequestID = "client-9"
	first, err := f.uc.Execute(tctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.BaseRef = "main"
	again, err := f.uc.Execute(tctx(), in)
	if err != nil || again.Created || again.Request.ID != first.Request.ID || again.Request.BaseRef != "release/1.4" {
		t.Fatalf("a retry returns the stored Request unchanged: %+v %v", again, err)
	}
}

func TestCreateWithinTx_BaseRef_ValidatedForEveryCaller(t *testing.T) {
	f := newCreateFixture()
	in := manualInput("child")
	in.BaseRef = "-bad"
	src, _ := domain.NormalizeSourceRef(in.Source)
	err := f.s.InTx(tctx(), func(ctx context.Context) error {
		_, err := f.uc.CreateWithinTx(ctx, in, src, testTenant, testReporter, domain.IdempotencyKey{}, false)
		return err
	})
	mustCode(t, err, "REQUEST_BASE_REF_INVALID")
}
