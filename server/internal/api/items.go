package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/links"
)

const (
	maxDescription = 2000
	defaultLimit   = 30
	maxLimit       = 100
)

type categoryJSON struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type itemJSON struct {
	ID          uuid.UUID     `json:"id"`
	URL         string        `json:"url"`
	Platform    string        `json:"platform"`
	Description *string       `json:"description"`
	Category    *categoryJSON `json:"category"`
	// CategorySource is "auto" while the classifier picks the category and
	// "user" once the user chose one.
	CategorySource string `json:"categorySource"`
	// Classifying is true while a new description waits for a category.
	Classifying bool `json:"classifying"`
	// WaitingForCredits is true when the item is waiting only because the
	// user is out of credits; it's sorted after a top-up.
	WaitingForCredits bool      `json:"waitingForCredits"`
	PreviewTitle      *string   `json:"previewTitle"`
	PreviewImage      *string   `json:"previewImage"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// GetItemRow and ListItemsRow have the same columns.
func toItemJSON(r db.GetItemRow) itemJSON {
	it := itemJSON{
		ID: r.ID, URL: r.Url, Platform: r.Platform, Description: r.Description,
		PreviewTitle: r.PreviewTitle, PreviewImage: r.PreviewImage,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
		CategorySource: r.CategorySource, Classifying: r.Classifying,
	}
	if r.CategoryID != nil && r.CategoryName != nil {
		it.Category = &categoryJSON{ID: *r.CategoryID, Name: *r.CategoryName}
	}
	return it
}

// cleanDescription trims the text; blank means "no description yet".
func cleanDescription(s *string) (*string, bool) {
	if s == nil {
		return nil, true
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil, true
	}
	if utf8.RuneCountInString(t) > maxDescription {
		return nil, false
	}
	return &t, true
}

type createItemRequest struct {
	// URL may be a bare link or the text a share sheet passes, such as
	// "Check out this video! https://...".
	URL         string  `json:"url"`
	Description *string `json:"description"`
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	var req createItemRequest
	if !decode(w, r, &req) {
		return
	}
	norm, platform, err := links.Normalize(req.URL)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_url", "Eso no parece un link.")
		return
	}
	if len(norm) > 2048 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_url", "Ese link es demasiado largo.")
		return
	}
	desc, ok := cleanDescription(req.Description)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "description_too_long", "La descripción tiene que tener menos de 2000 caracteres.")
		return
	}

	row, err := s.q.UpsertItem(r.Context(), db.UpsertItemParams{
		UserID: user, Url: norm, Platform: platform, Description: desc,
	})
	if err != nil {
		s.internal(w, r, "upsert item", err)
		return
	}
	if row.NeedsClassifying {
		s.enqueueClassify(r, row.ID)
	}
	item, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: row.ID, UserID: user})
	if err != nil {
		s.internal(w, r, "get saved item", err)
		return
	}
	status := http.StatusOK
	if row.Inserted {
		status = http.StatusCreated
	}
	out := toItemJSON(item)
	s.markWaiting(r, &out)
	writeJSON(w, status, out)
}

type listItemsResponse struct {
	Items      []itemJSON `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	q := r.URL.Query()

	limit := defaultLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "bad_limit", "limit must be a positive number.")
			return
		}
		limit = min(n, maxLimit)
	}

	params := db.ListItemsParams{
		UserID:          user,
		OnlyUndescribed: q.Get("filter") == "undescribed",
		Lim:             int32(limit + 1), // one extra tells us whether there's a next page
	}
	if f := q.Get("filter"); f != "" && f != "undescribed" {
		writeError(w, http.StatusBadRequest, "bad_filter", `filter must be "undescribed" or omitted.`)
		return
	}
	if c := q.Get("categoryId"); c != "" {
		id, err := uuid.Parse(c)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_category", "Invalid categoryId.")
			return
		}
		params.CategoryID = &id
	}
	if c := q.Get("cursor"); c != "" {
		ts, id, err := decodeCursor(c)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_cursor", "Invalid cursor.")
			return
		}
		params.BeforeCreated, params.BeforeID = &ts, &id
	}

	rows, err := s.q.ListItems(r.Context(), params)
	if err != nil {
		s.internal(w, r, "list items", err)
		return
	}
	resp := listItemsResponse{Items: make([]itemJSON, 0, min(len(rows), limit))}
	for i, row := range rows {
		if i == limit {
			last := rows[limit-1]
			c := encodeCursor(last.CreatedAt, last.ID)
			resp.NextCursor = &c
			break
		}
		resp.Items = append(resp.Items, toItemJSON(db.GetItemRow(row)))
	}
	ptrs := make([]*itemJSON, len(resp.Items))
	for i := range resp.Items {
		ptrs[i] = &resp.Items[i]
	}
	s.markWaiting(r, ptrs...)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	item, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
		return
	}
	if err != nil {
		s.internal(w, r, "get item", err)
		return
	}
	out := toItemJSON(item)
	s.markWaiting(r, &out)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// Editable: "description" (null moves it back to the inbox) and
	// "categoryId" (null means "no category"). Raw messages tell a missing
	// field apart from an explicit null.
	var req map[string]json.RawMessage
	if !decode(w, r, &req) {
		return
	}
	rawDesc, hasDesc := req["description"]
	rawCat, hasCat := req["categoryId"]
	if len(req) == 0 || len(req) != btoi(hasDesc)+btoi(hasCat) {
		writeError(w, http.StatusBadRequest, "bad_fields", `Send "description" and/or "categoryId".`)
		return
	}

	// Validate everything before changing anything.
	var desc *string
	if hasDesc {
		var d *string
		if err := json.Unmarshal(rawDesc, &d); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "description must be a string or null.")
			return
		}
		var ok bool
		if desc, ok = cleanDescription(d); !ok {
			writeError(w, http.StatusUnprocessableEntity, "description_too_long", "La descripción tiene que tener menos de 2000 caracteres.")
			return
		}
	}
	var catID *uuid.UUID
	if hasCat {
		if err := json.Unmarshal(rawCat, &catID); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "categoryId must be a UUID or null.")
			return
		}
		if catID != nil {
			owned, err := s.q.CategoryBelongsToUser(r.Context(), db.CategoryBelongsToUserParams{ID: *catID, UserID: user})
			if err != nil {
				s.internal(w, r, "check category", err)
				return
			}
			if !owned {
				writeError(w, http.StatusUnprocessableEntity, "unknown_category", "Esa categoría no existe.")
				return
			}
		}
	}

	if hasDesc {
		needs, err := s.q.SetItemDescription(r.Context(), db.SetItemDescriptionParams{Description: desc, ID: id, UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
			return
		}
		if err != nil {
			s.internal(w, r, "update description", err)
			return
		}
		if needs {
			s.enqueueClassify(r, id)
		}
	}

	if hasCat {
		old, err := s.q.SetItemCategoryByUser(r.Context(), db.SetItemCategoryByUserParams{CategoryID: catID, ID: id, UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
			return
		}
		if err != nil {
			s.internal(w, r, "set category", err)
			return
		}
		// A correction is a labeled example for tuning the classifier.
		if old.Description != nil && !sameUUID(old.FromCategoryID, catID) {
			if err := s.q.InsertCategoryFeedback(r.Context(), db.InsertCategoryFeedbackParams{
				UserID: user, ItemID: id, FromCategoryID: old.FromCategoryID, ToCategoryID: catID, Description: *old.Description,
			}); err != nil {
				s.log.ErrorContext(r.Context(), "record category feedback", "err", err)
			}
		}
	}

	item, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
		return
	}
	if err != nil {
		s.internal(w, r, "get updated item", err)
		return
	}
	out := toItemJSON(item)
	s.markWaiting(r, &out)
	writeJSON(w, http.StatusOK, out)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := s.q.DeleteItem(r.Context(), db.DeleteItemParams{ID: id, UserID: user})
	if err != nil {
		s.internal(w, r, "delete item", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type categoryListJSON struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	ItemCount int32     `json:"itemCount"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	rows, err := s.q.ListCategories(r.Context(), user)
	if err != nil {
		s.internal(w, r, "list categories", err)
		return
	}
	out := make([]categoryListJSON, 0, len(rows))
	for _, c := range rows {
		out = append(out, categoryListJSON{ID: c.ID, Name: c.Name, ItemCount: c.ItemCount, CreatedAt: c.CreatedAt.UTC()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

type createCategoryRequest struct {
	Name string `json:"name"`
}

// createCategory lets the user make a category to move an item into.
func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	var req createCategoryRequest
	if !decode(w, r, &req) {
		return
	}
	name := strings.Join(strings.Fields(req.Name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		writeError(w, http.StatusUnprocessableEntity, "bad_name", "El nombre de la categoría va de 1 a 60 caracteres.")
		return
	}
	// No name embedding yet, so the duplicate-name check skips this one;
	// Jev still matches against it by name.
	row, err := s.q.InsertCategory(r.Context(), db.InsertCategoryParams{UserID: user, Name: name})
	if err != nil {
		s.internal(w, r, "create category", err)
		return
	}
	writeJSON(w, http.StatusCreated, categoryJSON{ID: row.ID, Name: row.Name})
}

// markWaiting flags items that wait for classification only because the
// user has no credits left. It reads the balance once, and only when some
// item is still waiting. Errors leave the flags unset.
func (s *Server) markWaiting(r *http.Request, items ...*itemJSON) {
	if s.credits == nil {
		return
	}
	waiting := false
	for _, it := range items {
		waiting = waiting || it.Classifying
	}
	if !waiting {
		return
	}
	user, _ := auth.UserFrom(r.Context())
	bal, err := s.q.GetCreditBalance(r.Context(), user)
	if err != nil || bal > 0 { // no row yet: a new user with starter credits coming
		return
	}
	for _, it := range items {
		it.WaitingForCredits = it.Classifying
	}
}

// enqueueClassify queues classification. A failure is logged, not returned:
// the save already succeeded and the periodic sweep picks the item up.
func (s *Server) enqueueClassify(r *http.Request, id uuid.UUID) {
	if s.enqueue == nil {
		return
	}
	if err := s.enqueue.EnqueueClassify(r.Context(), id); err != nil {
		s.log.ErrorContext(r.Context(), "enqueue classify", "item", id, "err", err)
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "No encontramos ese guardado.")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.log.ErrorContext(r.Context(), what, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Algo salió mal.")
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	ts, idStr, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, uuid.Nil, errors.New("malformed cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(idStr)
	return t, id, err
}
