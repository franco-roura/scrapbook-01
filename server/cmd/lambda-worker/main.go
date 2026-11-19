// Command lambda-worker classifies items and fetches link previews on AWS
// Lambda. The API invokes it asynchronously with {"itemId": "..."} or
// {"previewItemId": "..."}; an hourly schedule invokes it with
// {"sweep": true} to catch missed items and users who topped up.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/app"
	"github.com/aaugoustis/waitlist/server/internal/config"
	"github.com/aaugoustis/waitlist/server/internal/jobs"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	pool, err := app.LambdaPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	core := app.NewCore(cfg, pool, log)

	w := jobs.Worker{
		Classify: func(ctx context.Context, id uuid.UUID) error {
			res, err := core.Classifier.Run(ctx, id)
			if err != nil {
				return err
			}
			log.InfoContext(ctx, "classified", "item", id, "skipped", res.Skipped,
				"category", res.Category, "new_category", res.NewCategory, "changed", res.Changed)
			return nil
		},
		Preview: func(ctx context.Context, id uuid.UUID) error {
			p, err := core.Previews.Run(ctx, id)
			if err != nil {
				return err
			}
			log.InfoContext(ctx, "preview", "item", id, "found", !p.Empty(), "image", p.ImageURL != "")
			return nil
		},
		Sweep: func(ctx context.Context) (int, error) {
			// Stop early enough to finish the current item before the timeout.
			if dl, ok := ctx.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, dl.Add(-20*time.Second))
				defer cancel()
			}
			n, err := core.Sweep(ctx, 50)
			log.InfoContext(ctx, "sweep", "classified", n, "err", err)
			if err != nil {
				return n, err
			}
			p, perr := core.SweepPreviews(ctx, 20)
			log.InfoContext(ctx, "sweep", "previews", p, "err", perr)
			return n, perr
		},
	}
	lambda.Start(w.Handle)
}
