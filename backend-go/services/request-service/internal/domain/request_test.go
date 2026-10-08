package domain

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func appCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperrors.AppError
	if !asAppError(err, &ae) {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae.Code
}

func asAppError(err error, target **apperrors.AppError) bool {
	ae, ok := err.(*apperrors.AppError)
	if ok {
		*target = ae
	}
	return ok
}

func TestParseRequestType_All(t *testing.T) {
	if len(AllRequestTypes()) != 11 {
		t.Fatalf("want 11 types, got %d", len(AllRequestTypes()))
	}
	for _, rt := range AllRequestTypes() {
		got, err := ParseRequestType(string(rt))
		if err != nil || got != rt {
			t.Fatalf("ParseRequestType(%q) = %q, %v", rt, got, err)
		}
	}
	for _, bad := range []string{"", "foo", "Bug"} {
		_, err := ParseRequestType(bad)
		if err == nil || appCode(t, err) != "REQUEST_INVALID_TYPE" {
			t.Fatalf("ParseRequestType(%q) err = %v", bad, err)
		}
	}
}

func TestParseRequestStatus_All(t *testing.T) {
	if len(AllRequestStatuses()) != 12 {
		t.Fatalf("want 12 statuses, got %d", len(AllRequestStatuses()))
	}
	for _, rs := range AllRequestStatuses() {
		if got, err := ParseRequestStatus(string(rs)); err != nil || got != rs {
			t.Fatalf("ParseRequestStatus(%q) = %q, %v", rs, got, err)
		}
	}
	for _, bad := range []string{"", "bar", "done"} {
		if _, err := ParseRequestStatus(bad); err == nil || appCode(t, err) != "REQUEST_INVALID_STATUS" {
			t.Fatalf("ParseRequestStatus(%q) err = %v", bad, err)
		}
	}
}

func TestParseSizeUrgencyProvider_All(t *testing.T) {
	for _, s := range []string{"S", "M", "L"} {
		if _, err := ParseSize(s); err != nil {
			t.Fatalf("ParseSize(%q): %v", s, err)
		}
	}
	for _, bad := range []string{"", "XL", "s"} {
		if _, err := ParseSize(bad); err == nil || appCode(t, err) != "REQUEST_INVALID_SIZE" {
			t.Fatalf("ParseSize(%q) err = %v", bad, err)
		}
	}
	for _, u := range []string{"normal", "urgent"} {
		if _, err := ParseUrgency(u); err != nil {
			t.Fatalf("ParseUrgency(%q): %v", u, err)
		}
	}
	for _, bad := range []string{"", "x"} {
		if _, err := ParseUrgency(bad); err == nil || appCode(t, err) != "REQUEST_INVALID_URGENCY" {
			t.Fatalf("ParseUrgency(%q) err = %v", bad, err)
		}
	}
	for _, p := range []string{"jira", "github", "gitlab", "linear", "mcp", "manual", "webhook"} {
		if _, err := ParseSourceProvider(p); err != nil {
			t.Fatalf("ParseSourceProvider(%q): %v", p, err)
		}
	}
	for _, bad := range []string{"", "svn"} {
		if _, err := ParseSourceProvider(bad); err == nil || appCode(t, err) != "REQUEST_INVALID_SOURCE_PROVIDER" {
			t.Fatalf("ParseSourceProvider(%q) err = %v", bad, err)
		}
	}
}

func TestNewRequest_Validation(t *testing.T) {
	valid := NewRequestInput{
		TenantID: "t1", Title: "  Fix login  ", SourceProvider: "manual", ReporterID: "u1",
	}
	cases := []struct {
		name string
		mut  func(in *NewRequestInput)
		code string
	}{
		{"blank title", func(in *NewRequestInput) { in.Title = "  \t " }, "REQUEST_TITLE_REQUIRED"},
		{"missing tenant", func(in *NewRequestInput) { in.TenantID = "" }, "REQUEST_TENANT_REQUIRED"},
		{"missing reporter", func(in *NewRequestInput) { in.ReporterID = "" }, "REQUEST_REPORTER_REQUIRED"},
		{"unknown provider", func(in *NewRequestInput) { in.SourceProvider = "svn" }, "REQUEST_INVALID_SOURCE_PROVIDER"},
		{"empty provider", func(in *NewRequestInput) { in.SourceProvider = "" }, "REQUEST_INVALID_SOURCE_PROVIDER"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := valid
			c.mut(&in)
			_, err := NewRequest(in)
			if err == nil || appCode(t, err) != c.code {
				t.Fatalf("want code %s, got %v", c.code, err)
			}
		})
	}

	r, err := NewRequest(valid)
	if err != nil {
		t.Fatalf("valid input: %v", err)
	}
	if r.Status != RequestStatusNew || r.Version != 1 || r.Urgency != UrgencyNormal {
		t.Fatalf("defaults wrong: %+v", r)
	}
	if r.Title != "Fix login" || r.ID == "" || r.HasType() {
		t.Fatalf("fields wrong: %+v", r)
	}
	if !r.CreatedAt.Equal(r.UpdatedAt) {
		t.Fatalf("created_at and updated_at should start equal")
	}
}

func TestRequestStatus_IsTerminal(t *testing.T) {
	for _, rs := range AllRequestStatuses() {
		want := rs == RequestStatusCompleted || rs == RequestStatusCancelled
		if rs.IsTerminal() != want {
			t.Fatalf("%s IsTerminal = %v, want %v", rs, rs.IsTerminal(), want)
		}
	}
}

func TestNewRequestLink_SelfRejected(t *testing.T) {
	if _, err := NewRequestLink("a", "a", LinkReasonRelatesTo, "u"); err == nil || appCode(t, err) != "REQUEST_LINK_SELF" {
		t.Fatalf("self link err = %v", err)
	}
	l, err := NewRequestLink("a", "b", LinkReasonBlocks, "u")
	if err != nil || l.ParentRequestID != "a" || l.ChildRequestID != "b" {
		t.Fatalf("link = %+v, %v", l, err)
	}
}

func TestErrorsMapToGRPC(t *testing.T) {
	cases := []struct {
		err  error
		code codes.Code
	}{
		{ErrRequestNotFound("x"), codes.NotFound},
		{ErrSolutionNotFound("x"), codes.NotFound},
		{ErrRequestTenantRequired(), codes.InvalidArgument},
		{ErrRequestReporterRequired(), codes.InvalidArgument},
		{ErrRequestInvalidType("x"), codes.InvalidArgument},
		{ErrRequestInvalidStatus("x"), codes.InvalidArgument},
		{ErrRequestInvalidSize("x"), codes.InvalidArgument},
		{ErrRequestInvalidUrgency("x"), codes.InvalidArgument},
		{ErrRequestInvalidSourceProvider("x"), codes.InvalidArgument},
		{ErrRequestTitleRequired(), codes.InvalidArgument},
		{ErrRequestLinkSelf(), codes.InvalidArgument},
		{ErrRequestVersionConflict("x", 1), codes.FailedPrecondition},
		{ErrSolutionVersionConflict("x", 1), codes.FailedPrecondition},
		{ErrSourceAlreadyExists("x"), codes.AlreadyExists},
	}
	for _, c := range cases {
		if got := status.Code(apperrors.ToGRPCStatus(c.err)); got != c.code {
			t.Errorf("%v: code %v, want %v", c.err, got, c.code)
		}
	}
	if !strings.Contains(ErrSourceAlreadyExists("abc").Error(), "abc") {
		t.Errorf("ErrSourceAlreadyExists should carry the existing request id")
	}
}
