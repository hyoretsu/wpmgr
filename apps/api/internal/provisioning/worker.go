package provisioning

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mosamlife/wpmgr/apps/api/internal/cryptbox"
	"github.com/mosamlife/wpmgr/apps/api/internal/db"
	"github.com/riverqueue/river"
	"time"
)

type Args struct {
	ID       uuid.UUID `json:"id"`
	TenantID uuid.UUID `json:"tenant_id"`
}

func (Args) Kind() string                 { return "provisioning_run" }
func (Args) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: Queue, MaxAttempts: 25} }

type Worker struct {
	river.WorkerDefaults[Args]
	repo     *Repo
	box      *cryptbox.AgeIdentity
	executor Executor
}

func NewWorker(repo *Repo, box *cryptbox.AgeIdentity, executor Executor) *Worker {
	return &Worker{repo: repo, box: box, executor: executor}
}
func (*Worker) Timeout(*river.Job[Args]) time.Duration { return 30 * time.Minute }
func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	conn, e := w.repo.pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	var locked bool
	e = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext('provisioning'),hashtext($1))`, job.Args.ID.String()).Scan(&locked)
	if e != nil {
		return e
	}
	if !locked {
		return river.JobSnooze(time.Second * 10)
	}
	defer func() {
		cleanup, cancel := db.CleanupContext(ctx)
		defer cancel()
		_, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock(hashtext('provisioning'),hashtext($1))`, job.Args.ID.String())
		if err != nil {
			// A session holding a lock cannot return to the shared pool.
			_ = conn.Hijack().Close(cleanup)
		}
	}()
	var record Record
	var secret []byte
	e = w.repo.pool.InTenantTx(ctx, job.Args.TenantID, func(tx pgx.Tx) error {
		var err error
		record, secret, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM site_provisioning WHERE tenant_id=$1 AND id=$2`, job.Args.TenantID, job.Args.ID))
		return err
	})
	if e != nil {
		return e
	}
	if record.Status == "complete" {
		return nil
	}
	update := func(status, step, code string) error {
		return w.repo.pool.InTenantTx(ctx, record.TenantID, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE site_provisioning SET status=$3,step=$4,error_code=nullif($5,''),updated_at=now() WHERE tenant_id=$1 AND id=$2`, record.TenantID, record.ID, status, step, code)
			return err
		})
	}
	if e = update("running", record.Step, ""); e != nil {
		return e
	}
	creds, e := decryptCredentials(w.box, secret)
	if e == nil {
		e = w.executor.Run(ctx, record, creds, func(step string) error {
			if step == "complete" {
				return errors.New("executor cannot mark complete")
			}
			record.Step = step
			return update("running", step, "")
		})
	}
	if e != nil {
		code := "provisioning_step_failed"
		var coded interface{ Code() string }
		if errors.As(e, &coded) && safeErrorCode(coded.Code()) {
			code = coded.Code()
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		writeErr := w.repo.pool.InTenantTx(cleanup, record.TenantID, func(tx pgx.Tx) error {
			_, err := tx.Exec(cleanup, `UPDATE site_provisioning SET status='failed',error_code=$3,updated_at=now() WHERE tenant_id=$1 AND id=$2`, record.TenantID, record.ID, code)
			return err
		})
		if writeErr != nil {
			return errors.New("provisioning failure persistence failed")
		}
		return nil
	}
	return update("complete", "complete", "")
}
