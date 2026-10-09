package provisioning

import (
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Enabled          bool
	OperatorTenantID uuid.UUID
	Runtime          RuntimeConfig
}

func LoadConfig(hosted bool) (Config, error) {
	var c Config
	enabled := os.Getenv("WPMGR_PROVISIONING_ENABLED")
	if enabled == "" || enabled == "false" {
		return c, nil
	}
	if enabled != "true" {
		return c, fmt.Errorf("WPMGR_PROVISIONING_ENABLED must be true or false")
	}
	if hosted {
		return c, fmt.Errorf("provisioning requires a self-hosted installation")
	}
	c.Enabled = true
	var err error
	c.OperatorTenantID, err = uuid.Parse(os.Getenv("WPMGR_PROVISIONING_OPERATOR_TENANT_ID"))
	if err != nil || c.OperatorTenantID == uuid.Nil {
		return c, fmt.Errorf("invalid provisioning operator tenant")
	}
	get := func(key string) string { return strings.TrimSpace(os.Getenv("WPMGR_PROVISIONING_" + key)) }
	c.Runtime = RuntimeConfig{MariaDBAddress: get("MARIADB_ADDRESS"), ProxyNetwork: get("PROXY_NETWORK"), WordPressImage: get("WORDPRESS_IMAGE"), WPCLIImage: get("WPCLI_IMAGE"), AgentPackage: get("AGENT_PACKAGE"), ControlPlaneURL: get("CONTROL_PLANE_URL")}
	if c.Runtime.ProxyNetwork == "" {
		c.Runtime.ProxyNetwork = "proxy"
	}
	if c.Runtime.MariaDBAddress == "" {
		return c, fmt.Errorf("provisioning MariaDB address required")
	}
	if raw := get("MARIADB_TLS"); raw != "" {
		c.Runtime.MariaDBTLS, err = strconv.ParseBool(raw)
		if err != nil {
			return c, fmt.Errorf("invalid provisioning MariaDB TLS mode")
		}
	}
	for _, im := range []string{c.Runtime.WordPressImage, c.Runtime.WPCLIImage} {
		if im == "" || strings.HasSuffix(im, ":latest") || (!strings.Contains(im, ":") && !strings.Contains(im, "@sha256:")) {
			return c, fmt.Errorf("approved pinned provisioning images required")
		}
	}
	for _, raw := range []string{c.Runtime.AgentPackage, c.Runtime.ControlPlaneURL} {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return c, fmt.Errorf("provisioning URLs require HTTPS")
		}
	}
	return c, nil
}
