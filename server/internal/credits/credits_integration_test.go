//go:build integration

package credits_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/franco-roura/scrapbook-01/server/db/migrations"
	"github.com/franco-roura/scrapbook-01/server/internal/ai"
	"github.com/franco-roura/scrapbook-01/server/internal/credits"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

func setup(t *testing.T) *db.Queries {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	return db.New(pool)
}

func TestLedger(t *testing.T) {
	q := setup(t)
	ctx := context.Background()
	l := &credits.Ledger{Q: q, Starter: 100_000} // $0.10
	user := uuid.New()

	for range 2 { // starter credits are granted once
		if b, err := l.Balance(ctx, user); err != nil || b != 100_000 {
			t.Fatalf("balance = %d, %v", b, err)
		}
	}

	// $5 pack → $4 of budget; the same payment id twice credits once.
	b, ok, err := l.RecordPurchase(ctx, user, 5, "starter", "pi_123")
	if err != nil || !ok || b != 4_100_000 {
		t.Fatalf("purchase: %d %v %v", b, ok, err)
	}
	b, ok, err = l.RecordPurchase(ctx, user, 5, "starter", "pi_123")
	if err != nil || ok || b != 4_100_000 {
		t.Fatalf("repeat purchase: %d %v %v", b, ok, err)
	}

	// Charging a meter subtracts its total and logs the calls.
	mctx, meter := ai.WithMeter(ctx)
	ai.RecordUsage(mctx, ai.Usage{Service: "test", Cost: 250})
	item := uuid.New()
	if err := l.Charge(ctx, user, "classify", &item, meter); err != nil {
		t.Fatal(err)
	}
	if b, _ := l.Balance(ctx, user); b != 4_099_750 {
		t.Fatalf("after charge: %d", b)
	}
	rows, err := q.ListCreditLedger(ctx, db.ListCreditLedgerParams{UserID: user, Lim: 10})
	if err != nil || len(rows) != 3 || rows[0].Kind != "usage" || rows[0].Amount != -250 {
		t.Fatalf("ledger: %+v %v", rows, err)
	}
	if avg, _ := q.AverageClassifyCost(ctx, user); avg != 250 {
		t.Fatalf("average classify cost = %v", avg)
	}

	// Spending past zero is allowed once (charged after the fact), then
	// CanSpend says no.
	empty := uuid.New()
	l0 := &credits.Ledger{Q: q, Starter: 100}
	mctx2, m := ai.WithMeter(ctx)
	ai.RecordUsage(mctx2, ai.Usage{Service: "test", Cost: 150})
	_ = l0.Charge(ctx, empty, "classify", nil, m)
	if okSpend, _ := l0.CanSpend(ctx, empty); okSpend {
		t.Fatal("negative balance can still spend")
	}
	if _, _, err := l.Grant(ctx, empty, 0, "nothing", nil); err == nil {
		t.Fatal("zero grant accepted")
	}
}
