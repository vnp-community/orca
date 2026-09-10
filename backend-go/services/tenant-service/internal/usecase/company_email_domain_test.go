package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func seedCompany(t *testing.T, companies *fakeCompanyRepository, id, name string) {
	t.Helper()
	c, err := domain.NewCompany(id, name, nil)
	if err != nil {
		t.Fatalf("building company: %v", err)
	}
	if _, err := companies.Create(context.Background(), c); err != nil {
		t.Fatalf("seeding company: %v", err)
	}
}

func TestAddCompanyEmailDomain_RegistersDomain(t *testing.T) {
	companies := newFakeCompanyRepository()
	domains := newFakeCompanyEmailDomainRepository()
	seedCompany(t, companies, "c1", "VNP")

	uc := NewAddCompanyEmailDomain(companies, domains)
	got, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "c1", EmailDomain: "VnPay.VN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EmailDomain != "vnpay.vn" {
		t.Errorf("email_domain = %q, want normalized %q", got.EmailDomain, "vnpay.vn")
	}
	companyID, found, err := domains.ResolveCompanyID(context.Background(), "vnpay.vn")
	if err != nil || !found || companyID != "c1" {
		t.Errorf("resolve after add = (%q, %v, %v), want (c1, true, nil)", companyID, found, err)
	}
}

func TestAddCompanyEmailDomain_UnknownCompanyFails(t *testing.T) {
	uc := NewAddCompanyEmailDomain(newFakeCompanyRepository(), newFakeCompanyEmailDomainRepository())
	if _, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "missing", EmailDomain: "vnpay.vn"}); err == nil {
		t.Fatal("expected an error for a non-existent company")
	}
}

func TestAddCompanyEmailDomain_RejectsDomainTakenByAnotherCompany(t *testing.T) {
	companies := newFakeCompanyRepository()
	domains := newFakeCompanyEmailDomainRepository()
	seedCompany(t, companies, "c1", "VNP")
	seedCompany(t, companies, "c2", "Softlink")

	uc := NewAddCompanyEmailDomain(companies, domains)
	if _, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "c1", EmailDomain: "vnpay.vn"}); err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}
	if _, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "c2", EmailDomain: "vnpay.vn"}); err == nil {
		t.Fatal("expected an error when a different company claims an already-registered domain")
	}
	// Re-registering under the SAME company must stay a no-op, not an error.
	if _, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "c1", EmailDomain: "vnpay.vn"}); err != nil {
		t.Errorf("re-registering the same (company, domain) pair should not error: %v", err)
	}
}

func TestAddCompanyEmailDomain_RejectsFullEmailAddress(t *testing.T) {
	companies := newFakeCompanyRepository()
	seedCompany(t, companies, "c1", "VNP")
	uc := NewAddCompanyEmailDomain(companies, newFakeCompanyEmailDomainRepository())

	if _, err := uc.Execute(context.Background(), AddCompanyEmailDomainInput{CompanyID: "c1", EmailDomain: "alice@vnpay.vn"}); err == nil {
		t.Fatal("expected an error for a full email address instead of a bare domain")
	}
}

func TestRemoveCompanyEmailDomain_IsIdempotent(t *testing.T) {
	domains := newFakeCompanyEmailDomainRepository()
	uc := NewRemoveCompanyEmailDomain(domains)

	if err := uc.Execute(context.Background(), RemoveCompanyEmailDomainInput{EmailDomain: "never-registered.example.com"}); err != nil {
		t.Errorf("removing an unregistered domain should not error: %v", err)
	}
}

func TestListCompanyEmailDomains_ReturnsOnlyThatCompanys(t *testing.T) {
	domains := newFakeCompanyEmailDomainRepository()
	_ = domains.Add(context.Background(), "c1", "vnpay.vn")
	_ = domains.Add(context.Background(), "c1", "vnpay.com.vn")
	_ = domains.Add(context.Background(), "c2", "softlink.com")

	uc := NewListCompanyEmailDomains(domains)
	got, err := uc.Execute(context.Background(), ListCompanyEmailDomainsInput{CompanyID: "c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d domains, want 2: %v", len(got), got)
	}
}

func TestResolveCompanyByEmailDomain_FoundAndNotFound(t *testing.T) {
	domains := newFakeCompanyEmailDomainRepository()
	_ = domains.Add(context.Background(), "c1", "vnpay.vn")
	uc := NewResolveCompanyByEmailDomain(domains)

	got, err := uc.Execute(context.Background(), ResolveCompanyByEmailDomainInput{EmailDomain: "VnPay.VN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Found || got.CompanyID != "c1" {
		t.Errorf("got %+v, want Found=true CompanyID=c1", got)
	}

	got, err = uc.Execute(context.Background(), ResolveCompanyByEmailDomainInput{EmailDomain: "unregistered.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Found {
		t.Errorf("expected Found=false for an unregistered domain, got %+v", got)
	}
}

func TestResolveCompanyByEmailDomain_RequiresDomain(t *testing.T) {
	uc := NewResolveCompanyByEmailDomain(newFakeCompanyEmailDomainRepository())
	if _, err := uc.Execute(context.Background(), ResolveCompanyByEmailDomainInput{EmailDomain: ""}); err == nil {
		t.Fatal("expected an error for an empty email_domain")
	}
}
