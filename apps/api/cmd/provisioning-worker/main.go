// provisioning-worker is the sole process granted same-host Docker and MariaDB
// provisioning access. It consumes only the provisioning River queue.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/config"
	"github.com/mosamlife/wpmgr/apps/api/internal/cryptbox"
	"github.com/mosamlife/wpmgr/apps/api/internal/db"
	"github.com/mosamlife/wpmgr/apps/api/internal/provisioning"
	"github.com/mosamlife/wpmgr/apps/api/internal/site"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type enrollment struct{ repo site.Repo }

func (e enrollment) Mint(ctx context.Context, tenantID, siteID uuid.UUID) (string, error) {
	return site.MintProvisioningEnrollmentCode(ctx, e.repo, tenantID, siteID)
}
func (e enrollment) Connected(ctx context.Context, tenantID, siteID uuid.UUID) (bool, error) {
	s, err := e.repo.Get(ctx, tenantID, siteID)
	return s.ConnectionState == site.StateConnected, err
}
func run(ctx context.Context) error {
	cfg, err := config.Load(os.Getenv("WPMGR_CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("worker configuration invalid")
	}
	pc, err := provisioning.LoadConfig(cfg.Hosted.Enabled)
	if err != nil || !pc.Enabled {
		return fmt.Errorf("provisioning worker requires explicit self-hosted configuration")
	}
	user, password := os.Getenv("WPMGR_PROVISIONING_MARIADB_USERNAME"), os.Getenv("WPMGR_PROVISIONING_MARIADB_PASSWORD")
	key := os.Getenv("WPMGR_SITE_DEST_AGE_SECRET")
	dir := os.Getenv("WPMGR_PROVISIONING_STATE_DIR")
	if user == "" || password == "" || key == "" || dir == "" {
		return fmt.Errorf("worker credentials and state directory required")
	}
	age, err := cryptbox.NewAgeIdentity(key)
	if err != nil {
		return fmt.Errorf("worker encryption identity invalid")
	}
	pool, err := db.ConnectApp(ctx, cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("worker database unavailable")
	}
	defer pool.Close()
	var bypass bool
	if err = pool.QueryRow(ctx, "SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(&bypass); err != nil || bypass {
		return fmt.Errorf("worker requires non-superuser non-BYPASSRLS application role")
	}
	workers := river.NewWorkers()
	repo := provisioning.NewRepo(pool, nil)
	runtime := provisioning.NewRuntime(dir, user, password, enrollment{repo: site.NewRepo(pool)})
	river.AddWorker(workers, provisioning.NewWorker(repo, age, runtime))
	client, err := river.NewClient(riverpgxv5.New(pool.Pool), &river.Config{Workers: workers, Queues: map[string]river.QueueConfig{provisioning.Queue: {MaxWorkers: 1}}})
	if err != nil {
		return fmt.Errorf("worker queue configuration invalid")
	}
	if err = client.Start(ctx); err != nil {
		return fmt.Errorf("worker queue unavailable")
	}
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(shutdown)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("provisioning worker stopped", "reason", err.Error())
		os.Exit(1)
	}
}
