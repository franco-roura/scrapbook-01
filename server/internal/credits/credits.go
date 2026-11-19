// Package credits keeps each user's prepaid AI budget. Amounts are
// micro-dollars of model cost. Packs are sold so that 80% of the price is
// model budget and 20% is margin, the split promised on the waitlist page.
package credits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/franco-roura/scrapbook-01/server/internal/ai"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

// ModelShare is the part of a pack's price that becomes model budget.
const ModelShare = 0.8

// MicroPerDollar converts dollars to ledger units.
const MicroPerDollar = 1_000_000

type Ledger struct {
	Q db.Querier
	// Starter is the free budget a new user gets, in micro-dollars.
	Starter int64
}

// Balance returns the user's balance, granting starter credits on first use.
func (l *Ledger) Balance(ctx context.Context, user uuid.UUID) (int64, error) {
	return l.Q.EnsureStarterCredits(ctx, db.EnsureStarterCreditsParams{UserID: user, Starter: l.Starter})
}

// CanSpend reports whether the user has any budget left. Usage is charged
// after the call, so a balance can dip slightly below zero.
func (l *Ledger) CanSpend(ctx context.Context, user uuid.UUID) (bool, error) {
	b, err := l.Balance(ctx, user)
	return b > 0, err
}

// Charge records what a meter collected. Zero-cost runs aren't recorded.
func (l *Ledger) Charge(ctx context.Context, user uuid.UUID, step string, item *uuid.UUID, m *ai.Meter) error {
	total := m.Total()
	if total <= 0 {
		return nil
	}
	detail, err := json.Marshal(map[string]any{"step": step, "calls": m.Records()})
	if err != nil {
		return err
	}
	_, err = l.Q.AddCredits(ctx, db.AddCreditsParams{UserID: user, Amount: -total, Kind: "usage", ItemID: item, Detail: detail})
	return err
}

// Grant adds budget by hand (support, promotions). ref, when set, makes the
// grant happen at most once.
func (l *Ledger) Grant(ctx context.Context, user uuid.UUID, micro int64, note string, ref *string) (int64, bool, error) {
	if micro <= 0 {
		return 0, false, errors.New("grant must be positive")
	}
	return l.add(ctx, user, micro, "grant", map[string]any{"note": note}, ref)
}

// RecordPurchase credits a sold pack: 80% of the price becomes budget. ref
// is the payment's id, so a payment notification processed twice credits
// once. The returned bool is false when ref was already recorded.
func (l *Ledger) RecordPurchase(ctx context.Context, user uuid.UUID, priceUSD float64, pack, ref string) (int64, bool, error) {
	if priceUSD <= 0 || ref == "" {
		return 0, false, errors.New("purchase needs a positive price and a payment reference")
	}
	micro := int64(math.Floor(priceUSD * ModelShare * MicroPerDollar))
	return l.add(ctx, user, micro, "purchase", map[string]any{"pack": pack, "priceUsd": priceUSD}, &ref)
}

func (l *Ledger) add(ctx context.Context, user uuid.UUID, micro int64, kind string, detail map[string]any, ref *string) (int64, bool, error) {
	if _, err := l.Balance(ctx, user); err != nil { // starter credits first
		return 0, false, err
	}
	d, err := json.Marshal(detail)
	if err != nil {
		return 0, false, err
	}
	bal, err := l.Q.AddCredits(ctx, db.AddCreditsParams{UserID: user, Amount: micro, Kind: kind, Detail: d, Reference: ref})
	if errors.Is(err, pgx.ErrNoRows) { // reference already used
		cur, err := l.Balance(ctx, user)
		return cur, false, err
	}
	if err != nil {
		return 0, false, fmt.Errorf("add credits: %w", err)
	}
	return bal, true, nil
}

// Dollars formats micro-dollars for people, e.g. 93_700 → 0.0937.
func Dollars(micro int64) float64 { return float64(micro) / MicroPerDollar }
