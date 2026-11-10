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
	ID           uuid.UUID     `json:"id"`
	URL          string        `json:"url"`
	Platform     string        `json:"platform"`
	Description  *string       `json:"description"`
	Category     *categoryJSON `json:"category"`
	PreviewTitle *string       `json:"previewTitle"`
	PreviewImage *string       `json:"previewImage"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// GetItemRow and ListItemsRow have the same columns.
func toItemJSON(r db.GetItemRow) itemJSON {
	it := itemJSON{
		ID: r.ID, URL: r.Url, Platform: r.Platform, Description: r.Description,
		PreviewTitle: r.PreviewTitle, PreviewImage: r.PreviewImage,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
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
		writeError(w, http.StatusUnprocessableEntity, "invalid_url", "That doesn't look like a link.")
		return
	}
	if len(norm) > 2048 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_url", "That link is too long.")
		return
	}
	desc, ok := cleanDescription(req.Description)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "description_too_long", "Keep the description under 2000 characters.")
		return
	}

	row, err := s.q.UpsertItem(r.Context(), db.UpsertItemParams{
		UserID: user, Url: norm, Platform: platform, Description: desc,
	})
	if err != nil {
		s.internal(w, r, "upsert item", err)
		return
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
	writeJSON(w, status, toItemJSON(item))
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
		writeError(w, http.StatusNotFound, "not_found", "Item not found.")
		return
	}
	if err != nil {
		s.internal(w, r, "get item", err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// Only description is editable for now. A raw message tells "missing"
	// apart from an explicit null (which clears it).
	var req map[string]json.RawMessage
	if !decode(w, r, &req) {
		return
	}
	raw, present := req["description"]
	if !present || len(req) != 1 {
		writeError(w, http.StatusBadRequest, "bad_fields", `Send exactly one field: "description".`)
		return
	}
	var d *string
	if err := json.Unmarshal(raw, &d); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "description must be a string or null.")
		return
	}
	desc, ok := cleanDescription(d)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "description_too_long", "Keep the description under 2000 characters.")
		return
	}

	n, err := s.q.SetItemDescription(r.Context(), db.SetItemDescriptionParams{Description: desc, ID: id, UserID: user})
	if err != nil {
		s.internal(w, r, "update item", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "Item not found.")
		return
	}
	item, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
	if err != nil {
		s.internal(w, r, "get updated item", err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
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
		writeError(w, http.StatusNotFound, "not_found", "Item not found.")
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

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Item not found.")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.log.ErrorContext(r.Context(), what, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
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
