package usecase

import (
	"context"
	"reflect"
	"testing"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestGetCredentialMetadataByOwner_NotFound_ReturnsFoundFalseNotError(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)

	uc := NewGetCredentialMetadataByOwner(metadataRepo)
	got, err := uc.Execute(context.Background(), GetCredentialMetadataByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "bitbucket",
	})
	if err != nil {
		t.Fatalf("expected no error for a not-yet-configured credential, got: %v", err)
	}
	if got.Found {
		t.Errorf("expected Found=false, got %+v", got)
	}
}

func TestGetCredentialMetadataByOwner_Found_ConfigJSONRoundTrips(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	store := newFakeSecretStore(rec)

	writeUC := NewWriteCredential(store, newFakeTxRunner(rec, metadataRepo, newFakeAuditRepo(rec)))
	created, err := writeUC.Execute(context.Background(), WriteCredentialInput{
		TenantID: "tenant-1", OwnerID: "bitbucket", Category: domain.CategoryScmOAuth,
		EncryptedEnvelope: []byte("tok-abc"), ConfigJSON: `{"baseUrl":"https://bitbucket.example.com"}`,
		RequestingService: "scm-integration-service",
	})
	if err != nil {
		t.Fatalf("seeding credential: %v", err)
	}

	uc := NewGetCredentialMetadataByOwner(metadataRepo)
	got, err := uc.Execute(context.Background(), GetCredentialMetadataByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "bitbucket",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Found {
		t.Fatal("expected Found=true")
	}
	if got.Metadata.ID != created.ID {
		t.Errorf("expected metadata id %q, got %q", created.ID, got.Metadata.ID)
	}
	if got.Metadata.ConfigJSON != `{"baseUrl":"https://bitbucket.example.com"}` {
		t.Errorf("expected ConfigJSON to round-trip, got %q", got.Metadata.ConfigJSON)
	}
}

func TestGetCredentialMetadataByOwner_MissingScope_Errors(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)

	uc := NewGetCredentialMetadataByOwner(metadataRepo)
	if _, err := uc.Execute(context.Background(), GetCredentialMetadataByOwnerInput{}); err == nil {
		t.Fatal("expected CREDENTIAL_MISSING_SCOPE error")
	}
}

// TestGetCredentialMetadataByOwnerResponse_NoSecretField enforces the same
// "no field capable of holding a secret" discipline
// credential-broker-service.md §9 documents for every other read-metadata
// RPC response.
func TestGetCredentialMetadataByOwnerResponse_NoSecretField(t *testing.T) {
	typ := reflect.TypeOf(credentialbrokerv1.GetCredentialMetadataByOwnerResponse{})
	assertNoBytesField(t, typ)
}

// assertNoBytesField walks typ's fields recursively (including pointer and
// embedded struct fields) and fails t if any field is a []byte (or other
// byte-slice) — the shape a leaked secret/ciphertext would take.
func assertNoBytesField(t *testing.T, typ reflect.Type) {
	t.Helper()
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return
		}
		if seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" {
				// Unexported field (e.g. protoimpl.MessageState's
				// unknownFields) — internal codegen plumbing, not a message
				// field a caller can populate or read.
				continue
			}
			fieldPath := path + "." + f.Name
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Uint8 {
				t.Errorf("field %s is a []byte — a metadata-only response must never carry a byte-slice field capable of holding a secret", fieldPath)
				continue
			}
			switch ft.Kind() {
			case reflect.Struct:
				walk(ft, fieldPath)
			case reflect.Slice, reflect.Array:
				walk(ft.Elem(), fieldPath+"[]")
			}
		}
	}
	walk(typ, typ.Name())
}
