package provisioning

import (
 "net/http"
 "github.com/gin-gonic/gin"
 "github.com/google/uuid"
 "github.com/mosamlife/wpmgr/apps/api/internal/authz"
 "github.com/mosamlife/wpmgr/apps/api/internal/cryptbox"
 "github.com/mosamlife/wpmgr/apps/api/internal/domain"
 "github.com/mosamlife/wpmgr/apps/api/internal/server/httpx"
)

type Handler struct{repo *Repo;box *cryptbox.AgeIdentity;config Config}
func NewHandler(repo *Repo,box *cryptbox.AgeIdentity,config Config)*Handler{return &Handler{repo:repo,box:box,config:config}}
func(h *Handler)Register(r *gin.RouterGroup){
 r.GET("/provisioning/config",h.getConfig)
 r.POST("/provisioning",authz.RequireOrgScope(),authz.RequirePermission(authz.PermSiteWrite),h.create)
 g:=r.Group("/sites/:siteId/provisioning",authz.RequireSiteAccess("siteId"))
 g.GET("",authz.RequirePermission(authz.PermSiteRead),h.status)
 g.POST("/retry",authz.RequirePermission(authz.PermSiteWrite),h.retry)
 g.GET("/credentials",authz.RequirePermission(authz.PermSiteWrite),h.credentials)
}
func(h *Handler)getConfig(c *gin.Context){p,_:=domain.PrincipalFromContext(c.Request.Context());c.JSON(http.StatusOK,gin.H{"enabled":Authorize(p,h.config)==nil})}
func(h *Handler)principal(c *gin.Context)(domain.Principal,bool){p,_:=domain.PrincipalFromContext(c.Request.Context());if e:=Authorize(p,h.config);e!=nil{httpx.Error(c,e);return p,false};return p,true}
func siteID(c *gin.Context)(uuid.UUID,bool){id,e:=uuid.Parse(c.Param("siteId"));if e!=nil{httpx.Error(c,domain.Validation("invalid_site_id","Invalid site ID"));return id,false};return id,true}
func(h *Handler)create(c *gin.Context){p,ok:=h.principal(c);if !ok{return};var in CreateInput;if e:=c.ShouldBindJSON(&in);e!=nil{httpx.Error(c,domain.Validation("invalid_provisioning_input","Invalid provisioning request"));return};if e:=ValidateInput(&in);e!=nil{httpx.Error(c,e);return};creds,e:=GenerateCredentials();if e!=nil{httpx.Error(c,e);return};secret,e:=encryptCredentials(h.box,creds);if e!=nil{httpx.Error(c,e);return};out,e:=h.repo.Create(c.Request.Context(),p,in,h.config.Runtime,secret);if e!=nil{httpx.Error(c,e);return};c.JSON(http.StatusAccepted,out)}
func(h *Handler)status(c *gin.Context){p,ok:=h.principal(c);if !ok{return};id,ok:=siteID(c);if !ok{return};out,_,e:=h.repo.Get(c.Request.Context(),p.TenantID,id);if e!=nil{httpx.Error(c,e);return};c.JSON(http.StatusOK,out)}
func(h *Handler)retry(c *gin.Context){p,ok:=h.principal(c);if !ok{return};id,ok:=siteID(c);if !ok{return};out,e:=h.repo.Retry(c.Request.Context(),p,id);if e!=nil{httpx.Error(c,e);return};c.JSON(http.StatusAccepted,out)}
func(h *Handler)credentials(c *gin.Context){c.Header("Cache-Control","no-store");c.Header("Pragma","no-cache");p,ok:=h.principal(c);if !ok{return};id,ok:=siteID(c);if !ok{return};out,secret,e:=h.repo.Get(c.Request.Context(),p.TenantID,id);if e!=nil{httpx.Error(c,e);return};creds,e:=decryptCredentials(h.box,secret);if e!=nil{httpx.Error(c,domain.Internal("credential_unavailable","Credentials unavailable"));return};c.JSON(http.StatusOK,gin.H{"admin_username":out.AdminUsername,"admin_password":creds.AdminPassword,"login_url":"https://"+out.Domain+"/wp-admin/"})}
