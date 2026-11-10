//go:build integration

// Run with a database from docker-compose.yml:
//
//	TEST_DATABASE_URL=postgres://scrapbook:scrapbook@localhost:54329/scrapbook?sslmode=disable \
//	  go test -tags integration ./internal/db/
package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/aaugoustis/waitlist/server/db/migrations"
	"github.com/aaugoustis/waitlist/server/internal/db"
)

func setup(t *testing.T) (*db.Queries, *pgxpool.Pool) {
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
	// Down then up proves the Down section works and gives a clean schema.
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	return db.New(pool), pool
}

func ptr(s string) *string { return &s }

func TestUpsertKeepsOneRowPerLink(t *testing.T) {
	q, _ := setup(t)
	ctx := context.Background()
	user := uuid.New()

	a, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: "https://x.com/a/status/1", Platform: "x"})
	if err != nil || !a.Inserted {
		t.Fatalf("first save: %+v %v", a, err)
	}
	b, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: "https://x.com/a/status/1", Platform: "x", Description: ptr("a thread on pricing")})
	if err != nil || b.Inserted || b.ID != a.ID {
		t.Fatalf("second save: %+v %v", b, err)
	}
	// Saving again with no description keeps the existing one.
	if _, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: "https://x.com/a/status/1", Platform: "x"}); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetItem(ctx, db.GetItemParams{ID: a.ID, UserID: user})
	if err != nil || got.Description == nil || *got.Description != "a thread on pricing" {
		t.Fatalf("get: %+v %v", got, err)
	}

	// The same link for another user is a separate row they own.
	other := uuid.New()
	c, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: other, Url: "https://x.com/a/status/1", Platform: "x"})
	if err != nil || !c.Inserted || c.ID == a.ID {
		t.Fatalf("other user save: %+v %v", c, err)
	}
	if _, err := q.GetItem(ctx, db.GetItemParams{ID: a.ID, UserID: other}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user read: err = %v", err)
	}
	if n, _ := q.DeleteItem(ctx, db.DeleteItemParams{ID: a.ID, UserID: other}); n != 0 {
		t.Fatalf("other user deleted %d rows", n)
	}
}

func TestDescriptionConstraint(t *testing.T) {
	q, _ := setup(t)
	_, err := q.UpsertItem(context.Background(), db.UpsertItemParams{UserID: uuid.New(), Url: "https://example.com", Platform: "web", Description: ptr("")})
	if err == nil {
		t.Fatal("empty description was accepted; the API must send NULL instead")
	}
}

func TestListCursorWithEqualTimestamps(t *testing.T) {
	q, pool := setup(t)
	ctx := context.Background()
	user := uuid.New()

	for _, u := range []string{"https://e.com/1", "https://e.com/2", "https://e.com/3", "https://e.com/4"} {
		if _, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: u, Platform: "web"}); err != nil {
			t.Fatal(err)
		}
	}
	// Force identical timestamps so only the id tiebreak orders them.
	same := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `UPDATE items SET created_at = $1 WHERE user_id = $2`, same, user); err != nil {
		t.Fatal(err)
	}

	seen := map[uuid.UUID]bool{}
	params := db.ListItemsParams{UserID: user, Lim: 3}
	for range 5 {
		rows, err := q.ListItems(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			if seen[r.ID] {
				t.Fatalf("item %s returned twice", r.ID)
			}
			seen[r.ID] = true
		}
		last := rows[len(rows)-1]
		params.BeforeCreated, params.BeforeID = &last.CreatedAt, &last.ID
	}
	if len(seen) != 4 {
		t.Fatalf("saw %d items, want 4", len(seen))
	}

	// The undescribed filter.
	one := ""
	for id := range seen {
		one = id.String()
		break
	}
	if _, err := q.SetItemDescription(ctx, db.SetItemDescriptionParams{Description: ptr("described"), ID: uuid.MustParse(one), UserID: user}); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListItems(ctx, db.ListItemsParams{UserID: user, OnlyUndescribed: true, Lim: 10})
	if err != nil || len(rows) != 3 {
		t.Fatalf("undescribed: %d rows, %v", len(rows), err)
	}
}
