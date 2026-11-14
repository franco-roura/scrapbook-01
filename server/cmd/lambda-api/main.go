// Command lambda-api serves the API and MCP endpoint on AWS Lambda behind
// API Gateway (HTTP API). Classification runs in the worker Lambda.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

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
	enq, err := jobs.NewLambdaInvoker(ctx, cfg.WorkerFunctionName)
	if err != nil {
		log.Error("worker invoker", "err", err)
		os.Exit(1)
	}
	h, err := newHandler(ctx, cfg, log, enq)
	if err != nil {
		log.Error("startup", "err", err)
		os.Exit(1)
	}
	lambda.Start(httpadapter.NewV2(h).ProxyWithContext)
}

// newHandler runs once per Lambda instance (cold start); the pool and the
// token keys are reused by every request the instance serves.
func newHandler(ctx context.Context, cfg config.Config, log *slog.Logger, enq app.Enqueuer) (http.Handler, error) {
	pool, err := app.LambdaPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return app.NewCore(cfg, pool, log).Handler(ctx, enq)
}
