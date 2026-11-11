// Package classify files a described item into one of the user's categories
// and links it to similar items. It runs in a background job after a save.
//
// Cost order, cheapest first:
//  1. One embedding of the description (graph links, duplicate checks).
//  2. Jev picks among the user's existing categories (~$0.04 per million
//     input tokens, output free).
//  3. Only when nothing fits: Claude Haiku names a new category (~200 tokens
//     in, a few out).
//
// Re-saving a link without changing its description does none of this.
package classify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aaugoustis/waitlist/server/internal/ai"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
)

type Chooser interface {
	Choose(ctx context.Context, state, instructions string, options map[string]string) (ai.Choice, error)
}

type Embedder interface {
	Embed(ctx context.Context, inputs ...string) ([][]float32, error)
}

type Namer interface {
	Name(ctx context.Context, description string, existing []string) (string, error)
}

// Notifier tells the user where an item was filed. Failures are logged, not
// retried: a missed notification isn't worth classifying again.
type Notifier interface {
	CategoryAssigned(ctx context.Context, userID, itemID uuid.UUID, category string) error
}

// NoneOfThese is the extra option Jev gets so it can say no category fits.
const NoneOfThese = "None of these"

const jevInstructions = "Which of the user's scrapbook categories does this saved item belong in? " +
	"Answer \"" + NoneOfThese + "\" unless one category clearly fits."

type Classifier struct {
	Q      db.Querier
	Jev    Chooser  // nil: existing categories can't be matched, so none are assigned
	Embed  Embedder // nil: no graph links and no duplicate-name check
	Namer  Namer    // nil: no new categories are created
	Notify Notifier // nil: no notifications
	// Credits, when set, gates AI work on the user's balance and charges
	// what each run cost.
	Credits *credits.Ledger
	Log     *slog.Logger

	// Threshold is the Jev confidence needed to file into an existing category.
	Threshold float64
	// SameName is the name-embedding similarity above which a proposed new
	// category is treated as an existing one ("Recipes" vs "Cooking").
	SameName float64
	// Neighbors and MinSimilarity shape the item's graph links.
	Neighbors     int
	MinSimilarity float32
}

// Result says what Run did, for logs and tests.
type Result struct {
	Skipped     string // why nothing was done, if so
	Embedded    bool
	Category    string
	NewCategory bool
	Changed     bool // the item's category changed
}

// Run classifies one item. Errors worth retrying are returned; permanent
// ones (a refusal, an unusable answer) are logged and the item is marked done.
func (c *Classifier) Run(ctx context.Context, itemID uuid.UUID) (Result, error) {
	it, err := c.Q.GetItemForClassify(ctx, itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{Skipped: "deleted"}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if it.Description == nil || it.DescribedAt == nil {
		return Result{Skipped: "no description"}, nil
	}
	if it.ClassifiedAt != nil && it.ClassifiedAt.Equal(*it.DescribedAt) {
		return Result{Skipped: "already classified"}, nil
	}
	desc := *it.Description
	var res Result

	if c.Credits != nil {
		ok, err := c.Credits.CanSpend(ctx, it.UserID)
		if err != nil {
			return res, err
		}
		if !ok {
			// Left unclassified: the sweep retries once credits are added.
			return Result{Skipped: "no credits"}, nil
		}
		var meter *ai.Meter
		ctx, meter = ai.WithMeter(ctx)
		// Charge whatever was spent, even if a later step fails and retries.
		defer func() {
			if cerr := c.Credits.Charge(context.WithoutCancel(ctx), it.UserID, "classify", &it.ID, meter); cerr != nil {
				c.log().ErrorContext(ctx, "charge credits", "item", it.ID, "err", cerr)
			}
		}()
	}

	var vec []float32
	if c.Embed != nil {
		vecs, err := c.Embed.Embed(ctx, desc)
		if err != nil {
			return res, err
		}
		vec = vecs[0]
		if err := c.Q.SetItemEmbedding(ctx, db.SetItemEmbeddingParams{ID: it.ID, Embedding: ai.VectorLiteral(vec)}); err != nil {
			return res, err
		}
		res.Embedded = true
	}

	if it.CategorySource == "auto" {
		cat, isNew, err := c.pickCategory(ctx, it.UserID, desc)
		if err != nil {
			return res, err
		}
		if cat != nil {
			res.Category, res.NewCategory = cat.Name, isNew
			n, err := c.Q.AssignAutoCategory(ctx, db.AssignAutoCategoryParams{ID: it.ID, CategoryID: &cat.ID, DescribedAt: it.DescribedAt})
			if err != nil {
				return res, err
			}
			res.Changed = n == 1 && (it.CategoryID == nil || *it.CategoryID != cat.ID)
		}
	}

	if vec != nil {
		if err := c.Q.DeleteEdgesForItem(ctx, it.ID); err != nil {
			return res, err
		}
		if err := c.Q.InsertSimilarEdges(ctx, db.InsertSimilarEdgesParams{
			ItemID: it.ID, UserID: it.UserID, K: int32(c.Neighbors), MinSimilarity: c.MinSimilarity,
		}); err != nil {
			return res, err
		}
	}

	if _, err := c.Q.MarkClassified(ctx, db.MarkClassifiedParams{ID: it.ID, DescribedAt: it.DescribedAt}); err != nil {
		return res, err
	}

	if res.Changed && c.Notify != nil {
		if err := c.Notify.CategoryAssigned(ctx, it.UserID, it.ID, res.Category); err != nil {
			c.log().WarnContext(ctx, "notify failed", "item", it.ID, "err", err)
		}
	}
	return res, nil
}

type category struct {
	ID   uuid.UUID
	Name string
}

// pickCategory returns an existing category Jev is confident about, or a new
// one named by Haiku, or nil when neither is possible.
func (c *Classifier) pickCategory(ctx context.Context, userID uuid.UUID, desc string) (*category, bool, error) {
	cats, err := c.Q.ListCategoriesForClassify(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	if len(cats) > 0 {
		if c.Jev == nil {
			return nil, false, nil
		}
		options := make(map[string]string, len(cats)+1)
		byName := make(map[string]uuid.UUID, len(cats))
		for _, cat := range cats {
			if cat.Name == NoneOfThese {
				continue
			}
			options[cat.Name] = examplesHint(cat.Examples)
			byName[cat.Name] = cat.ID
		}
		options[NoneOfThese] = "The item doesn't clearly belong in any of the other categories."

		choice, err := c.Jev.Choose(ctx, desc, jevInstructions, options)
		if err != nil {
			var he *ai.HTTPError
			if errors.As(err, &he) && !he.Retryable() {
				c.log().WarnContext(ctx, "jev rejected request; leaving item uncategorized", "err", err)
				return nil, false, nil
			}
			return nil, false, err
		}
		if id, ok := byName[choice.Label]; ok && choice.Confidence >= c.Threshold {
			return &category{ID: id, Name: choice.Label}, false, nil
		}
	}

	if c.Namer == nil {
		return nil, false, nil
	}
	names := make([]string, len(cats))
	for i, cat := range cats {
		names[i] = cat.Name
	}
	name, err := c.Namer.Name(ctx, desc, names)
	if errors.Is(err, ai.ErrRefused) {
		c.log().WarnContext(ctx, "category namer declined; leaving item uncategorized")
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var nameVec *string
	if c.Embed != nil {
		vecs, err := c.Embed.Embed(ctx, name)
		if err != nil {
			return nil, false, err
		}
		lit := ai.VectorLiteral(vecs[0])
		nameVec = &lit
		similar, err := c.Q.FindSimilarCategory(ctx, db.FindSimilarCategoryParams{UserID: userID, Embedding: lit})
		switch {
		case err == nil && similar.Similarity >= c.SameName:
			return &category{ID: similar.ID, Name: similar.Name}, false, nil
		case err != nil && !errors.Is(err, pgx.ErrNoRows):
			return nil, false, err
		}
	}

	row, err := c.Q.InsertCategory(ctx, db.InsertCategoryParams{UserID: userID, Name: name, NameEmbedding: nameVec})
	if err != nil {
		return nil, false, fmt.Errorf("create category %q: %w", name, err)
	}
	return &category{ID: row.ID, Name: row.Name}, true, nil
}

// examplesHint describes a category to Jev by what the user put in it.
func examplesHint(examples []string) string {
	if len(examples) == 0 {
		return ""
	}
	clipped := make([]string, len(examples))
	for i, e := range examples {
		if r := []rune(e); len(r) > 120 {
			e = string(r[:120]) + "…"
		}
		clipped[i] = e
	}
	return "Recent items: " + strings.Join(clipped, " | ")
}

func (c *Classifier) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}
