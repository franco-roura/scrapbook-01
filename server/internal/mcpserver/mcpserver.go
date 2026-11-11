// Package mcpserver exposes a user's library to their own AI (Claude,
// ChatGPT, ...) over the Model Context Protocol. The AI's reasoning runs on
// the user's plan; this server only does cheap retrieval: one query
// embedding per search, or a keyword match when embeddings are off.
//
// Transport: Streamable HTTP, stateless (each request stands alone).
// Auth: OAuth 2.1 with Supabase Auth as the authorization server. Clients
// discover it from /.well-known/oauth-protected-resource (RFC 9728) and send
// Supabase access tokens as bearer tokens.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/aaugoustis/waitlist/server/internal/ai"
	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/graph"
	"github.com/aaugoustis/waitlist/server/internal/links"
)

// Path is where the MCP endpoint is mounted.
const Path = "/mcp"

const metadataPath = "/.well-known/oauth-protected-resource"

type Embedder interface {
	Embed(ctx context.Context, inputs ...string) ([][]float32, error)
}

type Enqueuer interface {
	EnqueueClassify(ctx context.Context, itemID uuid.UUID) error
}

type Options struct {
	Queries  db.Querier
	Verifier *auth.Verifier
	Embed    Embedder // optional: without it, search matches keywords
	Enqueuer Enqueuer // optional: classification after save_item
	// Credits, when set: semantic search is charged to the user, and users
	// out of credits get keyword search (free) instead.
	Credits *credits.Ledger
	Logger  *slog.Logger
	// PublicURL is the API's external origin, e.g. https://api.scrapbook01.app.
	PublicURL string
	// AuthServer is Supabase Auth's issuer, e.g. https://abc.supabase.co/auth/v1.
	AuthServer string
}

type Server struct {
	o   Options
	mcp *mcp.Server
}

func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	s := &Server{o: o}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: "scrapbook01", Title: "Scrapbook/01", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "The user's Scrapbook/01 library: posts and links they saved, each with a short note on what it's about, " +
			"filed into categories. Search it when the user's request could use something they saved before " +
			"(recipes, trips, ideas, tips). Prefer their saved items over general knowledge when relevant, and link to them.",
		Logger: o.Logger,
	})
	s.addTools()
	return s
}

// Handler serves the MCP endpoint and its OAuth metadata. Mount it at "/".
func (s *Server) Handler() http.Handler {
	resource := strings.TrimRight(s.o.PublicURL, "/") + Path
	meta := sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{s.o.AuthServer},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Scrapbook/01",
	})

	verify := func(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
		id, exp, err := s.o.Verifier.Verify(token)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
		}
		return &sdkauth.TokenInfo{UserID: id.String(), Expiration: exp}, nil
	}
	protect := sdkauth.RequireBearerToken(verify, &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: strings.TrimRight(s.o.PublicURL, "/") + metadataPath + Path,
		ClockSkew:           30 * time.Second,
	})
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       s.o.Logger,
	})

	mux := http.NewServeMux()
	mux.Handle(metadataPath, meta)
	mux.Handle(metadataPath+Path, meta) // RFC 9728 path-suffixed form
	mux.Handle(Path, protect(stream))
	return mux
}

// userFrom reads the signed-in user that RequireBearerToken put on the request.
func userFrom(req *mcp.CallToolRequest) (uuid.UUID, error) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return uuid.Nil, errors.New("not signed in")
	}
	return uuid.Parse(req.Extra.TokenInfo.UserID)
}

// ---- tools ----

type Result struct {
	ID       string  `json:"id"`
	Title    string  `json:"title" jsonschema:"what the user said the item is about, or its site when undescribed"`
	Category string  `json:"category,omitempty"`
	URL      string  `json:"url"`
	Platform string  `json:"platform"`
	SavedOn  string  `json:"savedOn" jsonschema:"date saved, YYYY-MM-DD"`
	Score    float64 `json:"score,omitempty" jsonschema:"similarity to the query, 0 to 1 (semantic search only)"`
}

type SearchInput struct {
	Query    string `json:"query" jsonschema:"what to look for, in the user's words, e.g. 'pasta recipes' or 'things to do in Montevideo'"`
	Category string `json:"category,omitempty" jsonschema:"only search this category (exact name)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max results, 1-10, default 5"`
}

type SearchOutput struct {
	Results []Result `json:"results"`
	Mode    string   `json:"mode" jsonschema:"semantic or keyword"`
}

type ItemInput struct {
	ID string `json:"id" jsonschema:"item id from search_library"`
}

type ItemOutput struct {
	Result
	Description *string  `json:"description"`
	Related     []Result `json:"related" jsonschema:"items most similar to this one"`
}

type NeighborsInput struct {
	ID    string `json:"id" jsonschema:"item id"`
	Depth int    `json:"depth,omitempty" jsonschema:"1 (default) for direct links, 2 to include links of links"`
}

type NeighborsOutput struct {
	Items      []Result `json:"items"`
	Categories []string `json:"categories"`
}

type SaveInput struct {
	URL  string `json:"url" jsonschema:"the link to save"`
	Note string `json:"note,omitempty" jsonschema:"what it's about, in the user's words; it's used to file the item"`
}

type SaveOutput struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Created bool   `json:"created" jsonschema:"false if the link was already saved (the note was updated)"`
}

func (s *Server) addTools() {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_library",
		Description: "Search the user's saved posts and links by meaning. Returns compact results; use get_item for details.",
		Annotations: readOnly,
	}, s.search)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_item",
		Description: "Get one saved item with its full note and the items most similar to it.",
		Annotations: readOnly,
	}, s.getItem)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_neighbors",
		Description: "List items linked to a saved item in the user's graph (similar items), 1 or 2 hops out, and their categories.",
		Annotations: readOnly,
	}, s.neighbors)

	f := false
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "save_item",
		Description: "Save a link to the user's library, with a short note on what it's about. Only when the user asks to save something.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &f, IdempotentHint: true},
	}, s.save)
}

func (s *Server) search(ctx context.Context, req *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return nil, SearchOutput{}, errors.New("query is required")
	}
	limit := in.Limit
	if limit < 1 || limit > 10 {
		limit = 5
	}
	var category *string
	if c := strings.TrimSpace(in.Category); c != "" {
		category = &c
	}

	out := SearchOutput{Results: []Result{}}
	semantic := s.o.Embed != nil
	if semantic && s.o.Credits != nil {
		if semantic, err = s.o.Credits.CanSpend(ctx, user); err != nil {
			return nil, out, err
		}
		if semantic {
			var meter *ai.Meter
			ctx, meter = ai.WithMeter(ctx)
			defer func() {
				if cerr := s.o.Credits.Charge(context.WithoutCancel(ctx), user, "search", nil, meter); cerr != nil {
					s.o.Logger.ErrorContext(ctx, "mcp search: charge credits", "err", cerr)
				}
			}()
		}
	}
	if semantic {
		vecs, err := s.o.Embed.Embed(ctx, q)
		if err == nil {
			rows, err := s.o.Queries.SearchItemsByVector(ctx, db.SearchItemsByVectorParams{
				UserID: user, Embedding: ai.VectorLiteral(vecs[0]), Category: category, Lim: int32(limit),
			})
			if err != nil {
				return nil, out, err
			}
			out.Mode = "semantic"
			for _, r := range rows {
				out.Results = append(out.Results, result(r.ID, r.Url, r.Platform, r.Description, r.CategoryName, r.CreatedAt, r.Score))
			}
			return nil, out, nil
		}
		s.o.Logger.WarnContext(ctx, "mcp search: embedding failed, using keywords", "err", err)
	}

	rows, err := s.o.Queries.SearchItemsByText(ctx, db.SearchItemsByTextParams{
		UserID: user, Words: likeWords(q), Category: category, Lim: int32(limit),
	})
	if err != nil {
		return nil, out, err
	}
	out.Mode = "keyword"
	for _, r := range rows {
		out.Results = append(out.Results, result(r.ID, r.Url, r.Platform, r.Description, r.CategoryName, r.CreatedAt, 0))
	}
	return nil, out, nil
}

func (s *Server) getItem(ctx context.Context, req *mcp.CallToolRequest, in ItemInput) (*mcp.CallToolResult, ItemOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, ItemOutput{}, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, ItemOutput{}, errors.New("unknown item id")
	}
	it, err := s.o.Queries.GetItem(ctx, db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ItemOutput{}, errors.New("unknown item id")
	}
	if err != nil {
		return nil, ItemOutput{}, err
	}
	out := ItemOutput{
		Result:      result(it.ID, it.Url, it.Platform, it.Description, it.CategoryName, it.CreatedAt, 0),
		Description: it.Description,
		Related:     []Result{},
	}
	local, err := s.neighborhood(ctx, user, id, 1)
	if err != nil {
		return nil, out, err
	}
	out.Related = local.Items
	if len(out.Related) > 5 {
		out.Related = out.Related[:5]
	}
	return nil, out, nil
}

func (s *Server) neighbors(ctx context.Context, req *mcp.CallToolRequest, in NeighborsInput) (*mcp.CallToolResult, NeighborsOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, NeighborsOutput{}, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, NeighborsOutput{}, errors.New("unknown item id")
	}
	depth := 1
	if in.Depth == 2 {
		depth = 2
	}
	out, err := s.neighborhood(ctx, user, id, depth)
	return nil, out, err
}

// neighborhood lists the items linked to id (strongest first) and the
// categories they're in.
func (s *Server) neighborhood(ctx context.Context, user, id uuid.UUID, depth int) (NeighborsOutput, error) {
	out := NeighborsOutput{Items: []Result{}, Categories: []string{}}
	items, err := s.o.Queries.ListGraphItems(ctx, db.ListGraphItemsParams{UserID: user, Lim: 2000})
	if err != nil {
		return out, err
	}
	edges, err := s.o.Queries.ListEdgesForUser(ctx, user)
	if err != nil {
		return out, err
	}
	full := graph.Build(items, edges, false)
	local, ok := graph.Neighborhood(full, id.String(), depth, 25)
	if !ok {
		return out, errors.New("unknown item id")
	}
	byID := map[string]db.ListGraphItemsRow{}
	for _, it := range items {
		byID[it.ID.String()] = it
	}
	weight := map[string]float32{}
	for _, l := range local.Links {
		if l.Kind != "similar" {
			continue
		}
		for _, end := range []string{l.Source, l.Target} {
			if end != id.String() && l.Weight > weight[end] {
				weight[end] = l.Weight
			}
		}
	}
	for _, n := range local.Nodes {
		switch {
		case n.Kind == "category":
			out.Categories = append(out.Categories, n.Label)
		case n.ID != id.String():
			it := byID[n.ID]
			out.Items = append(out.Items, result(it.ID, it.Url, it.Platform, it.Description, it.CategoryName, time.Time{}, float64(weight[n.ID])))
		}
	}
	// Strongest links first.
	for i := 1; i < len(out.Items); i++ {
		for j := i; j > 0 && out.Items[j].Score > out.Items[j-1].Score; j-- {
			out.Items[j], out.Items[j-1] = out.Items[j-1], out.Items[j]
		}
	}
	return out, nil
}

func (s *Server) save(ctx context.Context, req *mcp.CallToolRequest, in SaveInput) (*mcp.CallToolResult, SaveOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, SaveOutput{}, err
	}
	norm, platform, err := links.Normalize(in.URL)
	if err != nil || len(norm) > 2048 {
		return nil, SaveOutput{}, errors.New("that doesn't look like a link")
	}
	var note *string
	if n := strings.TrimSpace(in.Note); n != "" {
		if r := []rune(n); len(r) > 2000 {
			n = string(r[:2000])
		}
		note = &n
	}
	row, err := s.o.Queries.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: norm, Platform: platform, Description: note})
	if err != nil {
		return nil, SaveOutput{}, err
	}
	if row.NeedsClassifying && s.o.Enqueuer != nil {
		if err := s.o.Enqueuer.EnqueueClassify(ctx, row.ID); err != nil {
			s.o.Logger.ErrorContext(ctx, "mcp save: enqueue classify", "err", err)
		}
	}
	return nil, SaveOutput{ID: row.ID.String(), URL: norm, Created: row.Inserted}, nil
}

func result(id uuid.UUID, url, platform string, desc, category *string, saved time.Time, score float64) Result {
	r := Result{ID: id.String(), Title: graph.Label(desc, url), URL: url, Platform: platform, Score: round(score)}
	if category != nil {
		r.Category = *category
	}
	if !saved.IsZero() {
		r.SavedOn = saved.UTC().Format("2006-01-02")
	}
	return r
}

func round(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// likeWords splits a query into ILIKE-safe words (escaping % and _),
// dropping very short ones.
func likeWords(q string) []string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	var out []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		if len([]rune(w)) >= 3 {
			out = append(out, r.Replace(w))
		}
	}
	if len(out) == 0 {
		out = []string{r.Replace(strings.ToLower(strings.TrimSpace(q)))}
	}
	return out
}
