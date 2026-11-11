package api

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
)

// typicalSaveCost (micro-dollars) estimates saves left before a user has
// history: an embedding, a Jev call and an occasional Haiku naming.
const typicalSaveCost = 30

type creditEntryJSON struct {
	AmountUSD float64    `json:"amountUsd"`
	Kind      string     `json:"kind"`
	Step      string     `json:"step,omitempty"`
	ItemID    *uuid.UUID `json:"itemId,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type creditsJSON struct {
	// BalanceUSD is model budget left, in dollars.
	BalanceUSD float64 `json:"balanceUsd"`
	// SavesLeft roughly estimates how many more saves can be sorted.
	SavesLeft int `json:"savesLeft"`
	// ModelShare is the part of a pack's price that becomes budget (0.8).
	ModelShare float64           `json:"modelShare"`
	Recent     []creditEntryJSON `json:"recent"`
}

// getCredits serves GET /v1/credits: balance, an estimate of saves left and
// recent ledger entries.
func (s *Server) getCredits(w http.ResponseWriter, r *http.Request) {
	if s.credits == nil {
		writeError(w, http.StatusNotFound, "not_found", "Credits aren't enabled.")
		return
	}
	user, _ := auth.UserFrom(r.Context())
	bal, err := s.credits.Balance(r.Context(), user)
	if err != nil {
		s.internal(w, r, "credit balance", err)
		return
	}
	avg, err := s.q.AverageClassifyCost(r.Context(), user)
	if err != nil {
		s.internal(w, r, "average cost", err)
		return
	}
	if avg <= 0 {
		avg = typicalSaveCost
	}
	rows, err := s.q.ListCreditLedger(r.Context(), db.ListCreditLedgerParams{UserID: user, Lim: 20})
	if err != nil {
		s.internal(w, r, "credit ledger", err)
		return
	}
	out := creditsJSON{
		BalanceUSD: credits.Dollars(bal),
		SavesLeft:  int(math.Max(0, math.Floor(float64(bal)/avg))),
		ModelShare: credits.ModelShare,
		Recent:     make([]creditEntryJSON, 0, len(rows)),
	}
	for _, row := range rows {
		var d struct {
			Step string `json:"step"`
		}
		_ = json.Unmarshal(row.Detail, &d)
		out.Recent = append(out.Recent, creditEntryJSON{
			AmountUSD: credits.Dollars(row.Amount), Kind: row.Kind, Step: d.Step, ItemID: row.ItemID, CreatedAt: row.CreatedAt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
