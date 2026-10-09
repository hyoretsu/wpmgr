package site

import (
	"context"
	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"time"
)

// MintProvisioningEnrollmentCode issues a site-bound code for an existing
// pending site. The agent consumes it through the normal enrollment endpoint.
func MintProvisioningEnrollmentCode(ctx context.Context, repo Repo, tenantID, siteID uuid.UUID) (string, error) {
	st, e := repo.Get(ctx, tenantID, siteID)
	if e != nil {
		return "", e
	}
	if st.ConnectionState != StatePendingEnrollment {
		return "", domain.Conflict("site_not_pending", "Site is not pending enrollment")
	}
	plaintext, e := generatePairingCode()
	if e != nil {
		return "", e
	}
	_, e = repo.MintSiteBoundCode(ctx, CreatePairingCodeInput{TenantID: tenantID, SiteName: st.Name}, siteID, hashPairingCode(plaintext), time.Now().Add(pairingCodeTTL))
	return plaintext, e
}
