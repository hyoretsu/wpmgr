package tests

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"github.com/mosamlife/wpmgr/apps/api/internal/provisioning"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"sync"
	"testing"
)

func TestProvisioningAtomicTenantIsolationAndRetry(t *testing.T) {
	app := startPostgres(t)
	owner := connectOwner(t, app)
	defer owner.Close()
	ctx := context.Background()
	migrator, e := rivermigrate.New(riverpgxv5.New(owner.Pool), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); e != nil {
		t.Fatal(e)
	}
	client, e := river.NewClient(riverpgxv5.New(app.Pool), &river.Config{})
	if e != nil {
		t.Fatal(e)
	}
	repo := provisioning.NewRepo(app, client)
	tenantA := seedTenant(t, app, "prov-a-"+uuid.NewString())
	tenantB := seedTenant(t, app, "prov-b-"+uuid.NewString())
	principal := domain.Principal{Type: domain.PrincipalUser, TenantID: tenantA, Role: "owner"}
	in := provisioning.CreateInput{Domain: uuid.NewString() + ".example.com", Title: "Provisioned title", AdminUsername: "admin_user", AdminEmail: "admin@example.com"}
	// Concurrent API submissions must create only one site and one job.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successes []provisioning.Record
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := repo.Create(ctx, principal, in, provisioning.RuntimeConfig{}, []byte("encrypted"))
			if e == nil {
				mu.Lock()
				successes = append(successes, r)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(successes) != 1 {
		t.Fatalf("expected exactly one successful concurrent request, got %d", len(successes))
	}
	record := successes[0]
	if _, _, e = repo.Get(ctx, tenantB, record.SiteID); e == nil {
		t.Fatal("cross-tenant record exposed")
	}
	if _, _, e = repo.Get(ctx, tenantA, record.SiteID); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.Retry(ctx, principal, record.SiteID); e == nil {
		t.Fatal("queued request retried")
	}
	if e = app.InTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE site_provisioning SET status='failed',step='install',error_code='installation_failed' WHERE id=$1`, record.ID)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	retry, e := repo.Retry(ctx, principal, record.SiteID)
	if e != nil {
		t.Fatal(e)
	}
	if retry.SiteID != record.SiteID || retry.Step != "install" || retry.Status != "queued" {
		t.Fatal("retry lost identity or progress")
	}
	_, secret, e := repo.Get(ctx, tenantA, record.SiteID)
	if e != nil || string(secret) != "encrypted" {
		t.Fatal("retry changed credentials", e)
	}
	// Explicitly target A under B's production helper: prove actual RLS gate.
	if e = app.InTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
		var n int
		e := tx.QueryRow(ctx, `SELECT count(*) FROM site_provisioning WHERE tenant_id=$1`, tenantA).Scan(&n)
		if e == nil && n != 0 {
			t.Fatal("RLS exposed foreign row")
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if e = app.InTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		var n int
		e := tx.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind=$1 AND args->>'id'=$2`, (provisioning.Args{}).Kind(), record.ID.String()).Scan(&n)
		if e == nil && n != 2 {
			t.Fatalf("expected initial and retry jobs only, got %d", n)
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
}
