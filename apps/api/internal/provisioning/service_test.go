package provisioning

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/cryptbox"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"strings"
	"testing"
)

func TestInputValidation(t *testing.T) {
	for _, value := range []string{"https://example.com", "example.com/path", "example.com:443", "../example.com", "-bad.example", "bad-.example", "127.0.0.1", "example..com", "localhost", "example.com;touch /tmp/foo"} {
		in := CreateInput{Domain: value, AdminUsername: "site_admin", AdminEmail: "admin@example.com"}
		if ValidateInput(&in) == nil {
			t.Errorf("accepted invalid domain %q", value)
		}
	}
	in := CreateInput{Domain: " EXAMPLE.COM ", AdminUsername: "site_admin", AdminEmail: "admin@example.com"}
	if e := ValidateInput(&in); e != nil {
		t.Fatal(e)
	}
	if in.Domain != "example.com" || in.Title != in.Domain {
		t.Fatal("domain normalization and title default failed")
	}
	in.AdminUsername = "admin;echo bad"
	if ValidateInput(&in) == nil {
		t.Fatal("unsafe username accepted")
	}
}
func TestOwnerAuthorization(t *testing.T) {
	tenant := uuid.New()
	p := domain.Principal{Type: domain.PrincipalUser, UserID: uuid.New(), TenantID: tenant, Role: "owner", Scope: domain.ScopeOrg}
	c := Config{Enabled: true, OperatorTenantID: tenant}
	if e := Authorize(p, c); e != nil {
		t.Fatal(e)
	}
	cases := []domain.Principal{p, p, p, p}
	cases[0].Type = domain.PrincipalAPIKey
	cases[1].Role = "admin"
	cases[2].TenantID = uuid.New()
	cases[3].Scope = domain.ScopeSite
	for _, p := range cases {
		if Authorize(p, c) == nil {
			t.Fatal("unauthorized principal accepted")
		}
	}
	c.Enabled = false
	if Authorize(p, c) == nil {
		t.Fatal("disabled provisioning accepted")
	}
}
func TestCredentialEncryptionAndRedaction(t *testing.T) {
	box, e := cryptbox.DeriveAgeIdentity([]byte("stable-test-session-seed"), "provisioning-test")
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := GenerateCredentials()
	if e != nil {
		t.Fatal(e)
	}
	cipher, e := encryptCredentials(box, credentials)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(cipher), credentials.AdminPassword) {
		t.Fatal("plaintext persisted")
	}
	recovered, e := decryptCredentials(box, cipher)
	if e != nil || recovered.AdminPassword != credentials.AdminPassword {
		t.Fatal("credential restart persistence failed", e)
	}
	record := Record{Config: RuntimeConfig{MariaDBAddress: "secret-host"}, TenantID: uuid.New()}
	b, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "secret-host") || strings.Contains(string(b), "admin_password") {
		t.Fatal("status exposed secrets or internal config")
	}
}
func TestDurableWorkerQueue(t *testing.T) {
	opts := (Args{}).InsertOpts()
	if opts.Queue != Queue || opts.MaxAttempts < 2 {
		t.Fatal("worker crashes must resume in dedicated queue")
	}
}
