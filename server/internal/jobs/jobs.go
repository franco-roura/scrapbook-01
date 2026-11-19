// Package jobs runs background work on River, a job queue stored in the same
// Postgres database, so there's no separate queue service to operate.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"github.com/franco-roura/scrapbook-01/server/internal/classify"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
	"github.com/franco-roura/scrapbook-01/server/internal/preview"
)

// ClassifyArgs asks for one item to be (re)classified.
type ClassifyArgs struct {
	ItemID uuid.UUID `json:"item_id"`
}

func (ClassifyArgs) Kind() string { return "classify_item" }

// One pending job per item is enough: the worker always reads the latest
// description. Completed jobs don't block a new one, so editing the
// description later classifies again.
func (ClassifyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 6,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
				rivertype.JobStateRetryable,
			},
		},
	}
}

type classifyWorker struct {
	river.WorkerDefaults[ClassifyArgs]
	c   *classify.Classifier
	log *slog.Logger
}

func (w *classifyWorker) Timeout(*river.Job[ClassifyArgs]) time.Duration { return 90 * time.Second }

func (w *classifyWorker) Work(ctx context.Context, job *river.Job[ClassifyArgs]) error {
	res, err := w.c.Run(ctx, job.Args.ItemID)
	if err != nil {
		return err
	}
	w.log.InfoContext(ctx, "classified", "item", job.Args.ItemID, "skipped", res.Skipped,
		"category", res.Category, "new_category", res.NewCategory, "changed", res.Changed)
	return nil
}

// PreviewArgs asks for one item's link preview to be fetched.
type PreviewArgs struct {
	ItemID uuid.UUID `json:"item_id"`
}

func (PreviewArgs) Kind() string { return "fetch_preview" }

// Previews are attempted once (the Runner records the attempt itself), so
// River only retries database failures, a few times.
func (PreviewArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type previewWorker struct {
	river.WorkerDefaults[PreviewArgs]
	r *preview.Runner
}

func (w *previewWorker) Timeout(*river.Job[PreviewArgs]) time.Duration { return 45 * time.Second }

func (w *previewWorker) Work(ctx context.Context, job *river.Job[PreviewArgs]) error {
	_, err := w.r.Run(ctx, job.Args.ItemID)
	return err
}

// sweepArgs re-queues items whose classification was missed (an enqueue that
// failed after a save, or jobs that ran out of attempts).
type sweepArgs struct{}

func (sweepArgs) Kind() string { return "sweep_unclassified" }

type sweepWorker struct {
	river.WorkerDefaults[sweepArgs]
	q      db.Querier
	client func() *river.Client[pgx.Tx]
}

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[sweepArgs]) error {
	ids, err := w.q.ItemsNeedingClassification(ctx, 200)
	if err != nil {
		return err
	}
	params := make([]river.InsertManyParams, len(ids))
	for i, id := range ids {
		params[i] = river.InsertManyParams{Args: ClassifyArgs{ItemID: id}}
	}
	if len(params) > 0 {
		if _, err := w.client().InsertMany(ctx, params); err != nil {
			return err
		}
	}
	pids, err := w.q.ItemsNeedingPreview(ctx, 50)
	if err != nil || len(pids) == 0 {
		return err
	}
	params = make([]river.InsertManyParams, len(pids))
	for i, id := range pids {
		params[i] = river.InsertManyParams{Args: PreviewArgs{ItemID: id}}
	}
	_, err = w.client().InsertMany(ctx, params)
	return err
}

// Queue wraps the River client: it enqueues jobs and, when workers are on,
// runs them.
type Queue struct {
	client  *river.Client[pgx.Tx]
	working bool
}

type Options struct {
	Pool       *pgxpool.Pool
	Queries    db.Querier
	Classifier *classify.Classifier
	Previews   *preview.Runner
	Logger     *slog.Logger
	// RunWorkers processes jobs in this process. With it off, the Queue only
	// enqueues (for an API-only deployment next to separate workers).
	RunWorkers  bool
	Concurrency int
}

// Migrate creates or updates River's own tables.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}

func New(o Options) (*Queue, error) {
	q := &Queue{working: o.RunWorkers}
	cfg := &river.Config{Logger: o.Logger}
	if o.RunWorkers {
		workers := river.NewWorkers()
		river.AddWorker(workers, &classifyWorker{c: o.Classifier, log: o.Logger})
		river.AddWorker(workers, &previewWorker{r: o.Previews})
		river.AddWorker(workers, &sweepWorker{q: o.Queries, client: func() *river.Client[pgx.Tx] { return q.client }})
		concurrency := o.Concurrency
		if concurrency <= 0 {
			concurrency = 4
		}
		cfg.Workers = workers
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: concurrency}}
		cfg.PeriodicJobs = []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(time.Minute),
				func() (river.JobArgs, *river.InsertOpts) { return sweepArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		}
	}
	client, err := river.NewClient(riverpgxv5.New(o.Pool), cfg)
	if err != nil {
		return nil, err
	}
	q.client = client
	return q, nil
}

// Start begins processing jobs (no-op without workers).
func (q *Queue) Start(ctx context.Context) error {
	if !q.working {
		return nil
	}
	return q.client.Start(ctx)
}

// Stop waits for running jobs to finish, up to ctx's deadline.
func (q *Queue) Stop(ctx context.Context) error {
	if !q.working {
		return nil
	}
	return q.client.Stop(ctx)
}

// EnqueueClassify implements api.Enqueuer.
func (q *Queue) EnqueueClassify(ctx context.Context, itemID uuid.UUID) error {
	_, err := q.client.Insert(ctx, ClassifyArgs{ItemID: itemID}, nil)
	return err
}

// EnqueuePreview implements api.Enqueuer.
func (q *Queue) EnqueuePreview(ctx context.Context, itemID uuid.UUID) error {
	_, err := q.client.Insert(ctx, PreviewArgs{ItemID: itemID}, nil)
	return err
}
