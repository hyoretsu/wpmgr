package provisioning

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateHandlerAcceptsOmittedTitle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenant := uuid.New()
	h := NewHandler(nil, nil, Config{Enabled: true, OperatorTenantID: tenant})
	router := gin.New()
	router.POST("/provisioning", func(c *gin.Context) {
		p := domain.Principal{Type: domain.PrincipalUser, UserID: uuid.New(), TenantID: tenant, Role: "owner"}
		c.Request = c.Request.WithContext(domain.WithPrincipal(c.Request.Context(), p))
		h.create(c)
	})
	request := httptest.NewRequest(http.MethodPost, "/provisioning", strings.NewReader(`{"domain":"example.com","admin_username":"admin_user","admin_email":"invalid-email"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "invalid_admin_email") {
		t.Fatalf("optional title failed generated JSON binding: %d %s", response.Code, response.Body.String())
	}
}
