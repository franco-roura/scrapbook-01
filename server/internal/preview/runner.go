package preview

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

// Querier is the part of db.Querier the Runner uses.
type Querier interface {
	GetItemForPreview(ctx context.Context, id uuid.UUID) (db.GetItemForPreviewRow, error)
	SetItemPreview(ctx context.Context, arg db.SetItemPreviewParams) error
}

// Runner fetches and stores one item's preview.
type Runner struct {
	Q     Querier
	Fetch interface {
		Fetch(ctx context.Context, pageURL, platform string) (Preview, error)
		Thumbnail(ctx context.Context, imageURL string) ([]byte, error)
	}
	// Store keeps the image copy. Nil (local development) keeps the
	// original image URL instead, which may expire.
	Store Store
	Log   *slog.Logger
}

// Run fetches the item's preview once. A link that can't be read is not an
// error: the attempt is recorded and the card shows without a preview. Only
// database failures are returned, so the job is retried for those alone.
func (r *Runner) Run(ctx context.Context, itemID uuid.UUID) (Preview, error) {
	it, err := r.Q.GetItemForPreview(ctx, itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Preview{}, nil // deleted meanwhile
	}
	if err != nil {
		return Preview{}, fmt.Errorf("preview: load item: %w", err)
	}
	if it.PreviewFetchedAt != nil {
		return Preview{}, nil
	}
	p, err := r.Fetch.Fetch(ctx, it.Url, it.Platform)
	if err != nil {
		r.Log.InfoContext(ctx, "preview: not found", "item", itemID, "err", err)
		p = Preview{}
	}
	image := ""
	if p.ImageURL != "" {
		image = p.ImageURL
		if r.Store != nil {
			image = r.copyImage(ctx, itemID, p.ImageURL)
		}
	}
	if err := r.Q.SetItemPreview(ctx, db.SetItemPreviewParams{
		ID: itemID, Title: p.Title, Description: p.Description, Site: p.Site, Image: image,
	}); err != nil {
		return p, fmt.Errorf("preview: save: %w", err)
	}
	return p, nil
}

// copyImage stores a thumbnail and returns its URL, or "" if that failed.
func (r *Runner) copyImage(ctx context.Context, itemID uuid.UUID, src string) string {
	thumb, err := r.Fetch.Thumbnail(ctx, src)
	if err != nil {
		r.Log.InfoContext(ctx, "preview: image skipped", "item", itemID, "err", err)
		return ""
	}
	u, err := r.Store.Put(ctx, "previews/"+itemID.String()+".jpg", thumb)
	if err != nil {
		r.Log.WarnContext(ctx, "preview: image not stored", "item", itemID, "err", err)
		return ""
	}
	return u
}
