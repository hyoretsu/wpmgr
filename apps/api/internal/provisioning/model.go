package provisioning

import (
 "context"
 "time"
 "github.com/google/uuid"
)

const Queue = "provisioning"
type RuntimeConfig struct {
 MariaDBAddress string `json:"mariadb_address"`
 MariaDBTLS bool `json:"mariadb_tls"`
 ProxyNetwork string `json:"proxy_network"`
 WordPressImage string `json:"wordpress_image"`
 WPCLIImage string `json:"wpcli_image"`
 AgentPackage string `json:"agent_package"`
 ControlPlaneURL string `json:"control_plane_url"`
}
type Record struct {
 ID uuid.UUID `json:"id"`
 SiteID uuid.UUID `json:"site_id"`
 TenantID uuid.UUID `json:"-"`
 Domain string `json:"domain"`
 Title string `json:"title"`
 AdminUsername string `json:"admin_username"`
 AdminEmail string `json:"admin_email"`
 Status string `json:"status"`
 Step string `json:"step"`
 ErrorCode string `json:"error_code,omitempty"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Config RuntimeConfig `json:"-"`
}
type Credentials struct {
 AdminPassword string `json:"admin_password"`
 DatabasePassword string `json:"database_password"`
 Salts map[string]string `json:"salts"`
}
type CreateInput struct {
 Domain string `json:"domain"`
 Title string `json:"title"`
 AdminUsername string `json:"admin_username"`
 AdminEmail string `json:"admin_email"`
}
type Executor interface { Run(context.Context, Record, Credentials, func(string) error) error }
type Enrollment interface {
 Mint(context.Context, uuid.UUID, uuid.UUID) (string,error)
 Connected(context.Context, uuid.UUID, uuid.UUID) (bool,error)
}
