package provisioning

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/mosamlife/wpmgr/apps/api/internal/cryptbox"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"net"
	"net/mail"
	"regexp"
	"strings"
)

var domainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var username = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,60}$`)

func ValidateInput(in *CreateInput) error {
	in.Domain = strings.ToLower(strings.TrimSpace(in.Domain))
	in.Title = strings.TrimSpace(in.Title)
	if len(in.Domain) > 253 || net.ParseIP(in.Domain) != nil || !strings.Contains(in.Domain, ".") {
		return domain.Validation("invalid_domain", "Provide a bare public domain")
	}
	for _, label := range strings.Split(in.Domain, ".") {
		if !domainLabel.MatchString(label) {
			return domain.Validation("invalid_domain", "Provide a bare public domain")
		}
	}
	if in.Title == "" {
		in.Title = in.Domain
	}
	if len(in.Title) > 200 {
		return domain.Validation("invalid_title", "Site title exceeds limit")
	}
	if !username.MatchString(in.AdminUsername) {
		return domain.Validation("invalid_admin_username", "Use letters, digits, or . _ @ -")
	}
	address, e := mail.ParseAddress(in.AdminEmail)
	if e != nil || address.Address != in.AdminEmail || len(in.AdminEmail) > 254 {
		return domain.Validation("invalid_admin_email", "Provide a valid email address")
	}
	return nil
}
func Authorize(p domain.Principal, c Config) error {
	if !c.Enabled {
		return domain.Unavailable("provisioning_disabled", "Provisioning is disabled")
	}
	if p.Type != domain.PrincipalUser || p.UserID == [16]byte{} || p.TenantID != c.OperatorTenantID || p.Role != "owner" || p.Scope == domain.ScopeSite {
		return domain.Forbidden("provisioning_owner_required", "Provisioning requires the host operator organization's owner")
	}
	return nil
}
func GenerateCredentials() (Credentials, error) {
	out := Credentials{Salts: map[string]string{}}
	secret := func() (string, error) {
		b := make([]byte, 48)
		if _, e := rand.Read(b); e != nil {
			return "", e
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	var err error
	out.AdminPassword, err = secret()
	if err != nil {
		return out, err
	}
	out.DatabasePassword, err = secret()
	if err != nil {
		return out, err
	}
	for _, k := range []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"} {
		out.Salts[k], err = secret()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
func encryptCredentials(box *cryptbox.AgeIdentity, c Credentials) ([]byte, error) {
	b, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	return box.Encrypt(b)
}
func decryptCredentials(box *cryptbox.AgeIdentity, b []byte) (Credentials, error) {
	var c Credentials
	plain, e := box.Decrypt(b)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(plain, &c)
	return c, e
}

var errorCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func safeErrorCode(value string) bool { return errorCodePattern.MatchString(value) }
