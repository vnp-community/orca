package httpwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	tenantA  = "11111111-1111-1111-1111-111111111111"
	reporter = "22222222-2222-2222-2222-222222222222"
	project  = "33333333-3333-3333-3333-333333333333"
)

type fakeCreator struct {
	seen    map[string]string // ref -> request id
	inputs  []usecase.CreateRequestInput
	ctxUser string
	ctxTen  string
	err     error
}

func (f *fakeCreator) Execute(ctx context.Context, in usecase.CreateRequestInput) (usecase.CreateRequestResult, error) {
	f.ctxTen, _ = tenant.TenantID(ctx)
	f.ctxUser, _ = tenant.UserID(ctx)
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return usecase.CreateRequestResult{}, f.err
	}
	if id, ok := f.seen[in.Source.Ref]; ok {
		return usecase.CreateRequestResult{Request: domain.Request{ID: id}}, nil
	}
	id := "req-" + in.Source.Ref
	f.seen[in.Source.Ref] = id
	return usecase.CreateRequestResult{Request: domain.Request{ID: id}, Created: true}, nil
}

func sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func setup(t *testing.T) (*http.ServeMux, *fakeCreator) {
	t.Helper()
	srcs, err := ParseStaticSources(`[{"tenant_id":"`+tenantA+`","source":"sentry","reporter_id":"`+reporter+`","secret_env":"S"}]`,
		func(string) string { return "topsecret" }, nil)
	if err != nil {
		t.Fatal(err)
	}
	cr := &fakeCreator{seen: map[string]string{}}
	mux := http.NewServeMux()
	NewHandler(srcs, cr).Mount(mux)
	return mux, cr
}

func post(mux *http.ServeMux, source, tenantID, sig, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/request-webhooks/"+source, strings.NewReader(body))
	if tenantID != "" {
		req.Header.Set(HeaderTenant, tenantID)
	}
	if sig != "" {
		req.Header.Set(HeaderSignature, sig)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func goodBody(ref string) string {
	return `{"project_id":"` + project + `","ref":"` + ref + `","title":"Crash in checkout","body":"details","url":"https://x/1","hints":{"issue_type":"Bug","labels":["prod"]}}`
}

func TestRequestWebhook_GoodSignatureCreates(t *testing.T) {
	mux, cr := setup(t)
	body := goodBody("evt-1")
	rec := post(mux, "sentry", tenantA, sign("topsecret", body), body)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"created":true`) || !strings.Contains(rec.Body.String(), `"request_id":"req-evt-1"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	in := cr.inputs[0]
	if in.Source.Provider != domain.SourceProviderWebhook || in.Source.Site != "sentry" || in.Source.Ref != "evt-1" || in.Hints.IssueType != "Bug" || in.ActorKind != domain.ActorKindSystem || in.AllowTypeHint {
		t.Fatalf("%+v", in)
	}
	if cr.ctxTen != tenantA || cr.ctxUser != reporter {
		t.Fatalf("ctx tenant=%s user=%s", cr.ctxTen, cr.ctxUser)
	}
}

func TestRequestWebhook_ReplayReturnsCreatedFalse(t *testing.T) {
	mux, _ := setup(t)
	body := goodBody("evt-2")
	post(mux, "sentry", tenantA, sign("topsecret", body), body)
	rec := post(mux, "sentry", tenantA, sign("topsecret", body), body)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"created":false`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestRequestWebhook_AuthFailuresAreIndistinguishable401(t *testing.T) {
	mux, cr := setup(t)
	body := goodBody("evt-3")
	good := sign("topsecret", body)
	cases := map[string]*httptest.ResponseRecorder{
		"bad signature":        post(mux, "sentry", tenantA, sign("wrong", body), body),
		"missing signature":    post(mux, "sentry", tenantA, "", body),
		"malformed signature":  post(mux, "sentry", tenantA, "sha256=zz", body),
		"missing tenant":       post(mux, "sentry", "", good, body),
		"tenant not a uuid":    post(mux, "sentry", "tenant", good, body),
		"other tenant":         post(mux, "sentry", "44444444-4444-4444-4444-444444444444", good, body),
		"unknown source":       post(mux, "other", tenantA, good, body),
		"signature of changed": post(mux, "sentry", tenantA, good, body+" "),
	}
	var ref string
	for name, rec := range cases {
		if rec.Code != 401 {
			t.Errorf("%s: status %d", name, rec.Code)
		}
		if ref == "" {
			ref = rec.Body.String()
		} else if rec.Body.String() != ref {
			t.Errorf("%s: body %q differs from %q", name, rec.Body, ref)
		}
	}
	if len(cr.inputs) != 0 {
		t.Fatal("nothing may be created without authentication")
	}
}

func TestRequestWebhook_BodyOver256KiB413(t *testing.T) {
	mux, cr := setup(t)
	body := `{"title":"` + strings.Repeat("a", MaxBodyBytes) + `"}`
	rec := post(mux, "sentry", tenantA, sign("topsecret", body), body)
	if rec.Code != http.StatusRequestEntityTooLarge || len(cr.inputs) != 0 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRequestWebhook_RejectsUnknownFieldsAndBadJSON(t *testing.T) {
	mux, _ := setup(t)
	for _, body := range []string{`{"project_id":"x","extra":1}`, `nope`} {
		if rec := post(mux, "sentry", tenantA, sign("topsecret", body), body); rec.Code != 400 {
			t.Errorf("%q: %d", body, rec.Code)
		}
	}
}

func TestRequestWebhook_UseCaseErrorsMapToHTTP(t *testing.T) {
	mux, cr := setup(t)
	cr.err = domain.ErrRequestProjectRequired()
	body := goodBody("evt-4")
	rec := post(mux, "sentry", tenantA, sign("topsecret", body), body)
	if rec.Code != 400 || !bytes.Contains(rec.Body.Bytes(), []byte("REQUEST_PROJECT_REQUIRED")) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestParseStaticSources_Validation(t *testing.T) {
	env := func(string) string { return "" }
	for name, raw := range map[string]string{
		"bad tenant":    `[{"tenant_id":"x","source":"s","reporter_id":"` + reporter + `","secret_env":"S"}]`,
		"bad reporter":  `[{"tenant_id":"` + tenantA + `","source":"s","reporter_id":"x","secret_env":"S"}]`,
		"no source":     `[{"tenant_id":"` + tenantA + `","reporter_id":"` + reporter + `","secret_env":"S"}]`,
		"empty secret":  `[{"tenant_id":"` + tenantA + `","source":"s","reporter_id":"` + reporter + `","secret_env":"S"}]`,
		"no secret ref": `[{"tenant_id":"` + tenantA + `","source":"s","reporter_id":"` + reporter + `"}]`,
		"not json":      `{`,
	} {
		if _, err := ParseStaticSources(raw, env, nil); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	s, err := ParseStaticSources("", env, nil)
	if err != nil || s.Len() != 0 {
		t.Fatalf("empty config disables the route: %v", err)
	}
	s, err = ParseStaticSources(`[{"tenant_id":"`+tenantA+`","source":"s","reporter_id":"`+reporter+`","secret_file":"/x"}]`, env,
		func(string) ([]byte, error) { return []byte("filesecret\n"), nil })
	if err != nil {
		t.Fatal(err)
	}
	if src, ok := s.Resolve(tenantA, "s"); !ok || string(src.Secret) != "filesecret" {
		t.Fatalf("%+v %v", src, ok)
	}
}
