package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mosamlife/wpmgr/apps/api/internal/db"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"github.com/riverqueue/river"
)

type SiteCreationGate interface {
	CheckSiteCreate(context.Context, pgx.Tx, uuid.UUID) error
}
type Repo struct {
	pool   *db.Pool
	client *river.Client[pgx.Tx]
	gate   SiteCreationGate
}

func NewRepo(pool *db.Pool, client *river.Client[pgx.Tx]) *Repo {
	return &Repo{pool: pool, client: client}
}
func (r *Repo) SetSiteCreationGate(gate SiteCreationGate) { r.gate = gate }

const columns = `id,site_id,tenant_id,domain,title,admin_username,admin_email,status,step,coalesce(error_code,''),created_at,updated_at,config,encrypted_credentials`

func scan(row pgx.Row) (Record, []byte, error) {
	var r Record
	var config, secret []byte
	e := row.Scan(&r.ID, &r.SiteID, &r.TenantID, &r.Domain, &r.Title, &r.AdminUsername, &r.AdminEmail, &r.Status, &r.Step, &r.ErrorCode, &r.CreatedAt, &r.UpdatedAt, &config, &secret)
	if e == nil {
		e = json.Unmarshal(config, &r.Config)
	}
	return r, secret, e
}
func mapError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.NotFound("provisioning_not_found", "Provisioning record not found")
	}
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" {
		return domain.Conflict("provisioning_resource_exists", "Domain or site already exists")
	}
	return e
}
func (r *Repo) Create(ctx context.Context, p domain.Principal, in CreateInput, config RuntimeConfig, secret []byte) (Record, error) {
	var out Record
	err := r.pool.InTenantTxAsUser(ctx, p.TenantID, p.UserID, func(tx pgx.Tx) error {
		if r.gate != nil {
			if e := r.gate.CheckSiteCreate(ctx, tx, p.TenantID); e != nil {
				return e
			}
		}
		siteID := uuid.New()
		id := uuid.New()
		if _, e := tx.Exec(ctx, `INSERT INTO sites(id,tenant_id,url,name,status,connection_state,tags) VALUES($1,$2,$3,$4,'pending','pending_enrollment','{}')`, siteID, p.TenantID, "https://"+in.Domain, in.Title); e != nil {
			return e
		}
		b, e := json.Marshal(config)
		if e != nil {
			return e
		}
		out, _, e = scan(tx.QueryRow(ctx, `INSERT INTO site_provisioning(id,site_id,tenant_id,domain,title,admin_username,admin_email,config,encrypted_credentials) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+columns, id, siteID, p.TenantID, in.Domain, in.Title, in.AdminUsername, in.AdminEmail, b, secret))
		if e != nil {
			return e
		}
		_, e = r.client.InsertTx(ctx, tx, Args{ID: id, TenantID: p.TenantID}, nil)
		return e
	})
	return out, mapError(err)
}
func (r *Repo) Get(ctx context.Context, tenantID, siteID uuid.UUID) (Record, []byte, error) {
	var out Record
	var secret []byte
	e := r.pool.InTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var err error
		out, secret, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM site_provisioning WHERE tenant_id=$1 AND site_id=$2`, tenantID, siteID))
		return err
	})
	return out, secret, mapError(e)
}
func (r *Repo) Retry(ctx context.Context, p domain.Principal, siteID uuid.UUID) (Record, error) {
	var out Record
	e := r.pool.InTenantTxAsUser(ctx, p.TenantID, p.UserID, func(tx pgx.Tx) error {
		var err error
		out, _, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM site_provisioning WHERE tenant_id=$1 AND site_id=$2 FOR UPDATE`, p.TenantID, siteID))
		if err != nil {
			return err
		}
		if out.Status != "failed" {
			return domain.Conflict("provisioning_not_failed", "Only failed provisioning can be retried")
		}
		if _, err = tx.Exec(ctx, `UPDATE site_provisioning SET status='queued',error_code=NULL,updated_at=now() WHERE tenant_id=$1 AND id=$2`, p.TenantID, out.ID); err != nil {
			return err
		}
		_, err = r.client.InsertTx(ctx, tx, Args{ID: out.ID, TenantID: p.TenantID}, nil)
		out.Status = "queued"
		out.ErrorCode = ""
		return err
	})
	return out, mapError(e)
}
