// Package mcpserver exposes a user's library to their own AI (Claude,
// ChatGPT, ...) over the Model Context Protocol. The AI's reasoning runs on
// the user's plan; this server only does cheap retrieval: one query
// embedding per search, or a keyword match when embeddings are off.
//
// Transport: Streamable HTTP, stateless (each request stands alone).
// Auth: OAuth 2.1 with WorkOS AuthKit as the authorization server. Clients
// discover it from /.well-known/oauth-protected-resource (RFC 9728), register
// with AuthKit (CIMD or dynamic registration), and send AuthKit access tokens
// as bearer tokens.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/franco-roura/scrapbook-01/server/internal/ai"
	"github.com/franco-roura/scrapbook-01/server/internal/auth"
	"github.com/franco-roura/scrapbook-01/server/internal/credits"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
	"github.com/franco-roura/scrapbook-01/server/internal/graph"
	"github.com/franco-roura/scrapbook-01/server/internal/links"
	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

// Path is where the MCP endpoint is mounted.
const Path = "/mcp"

const metadataPath = "/.well-known/oauth-protected-resource"

type Embedder interface {
	Embed(ctx context.Context, inputs ...string) ([][]float32, error)
}

// Reranker picks among labeled options; *ai.Jev implements it.
type Reranker interface {
	Choose(ctx context.Context, state, instructions string, options map[string]string) (ai.Choice, error)
}

type Enqueuer interface {
	EnqueueClassify(ctx context.Context, itemID uuid.UUID) error
	// EnqueuePreview fetches a new link's preview (title, image).
	EnqueuePreview(ctx context.Context, itemID uuid.UUID) error
}

type Options struct {
	Queries  db.Querier
	Verifier *auth.Verifier
	Embed    Embedder // optional: without it, search matches keywords
	Enqueuer Enqueuer // optional: classification after save_item
	// Rerank, when set (Jev), reorders semantic results by how well each
	// matches the query. Optional.
	Rerank Reranker
	// Credits, when set: semantic search is charged to the user, and users
	// out of credits get keyword search (free) instead.
	Credits *credits.Ledger
	Logger  *slog.Logger
	// PublicURL is the API's external origin, e.g. https://api.scrapbook01.app.
	PublicURL string
	// AuthServer is the AuthKit domain, e.g. https://abc-123.authkit.app.
	AuthServer string
	// ReminderEvery is how long after the user's last analysis the assistant
	// is asked to remind them to analyze their new saves. Default 15 days.
	ReminderEvery time.Duration
	// Now is the clock, for tests. Default time.Now.
	Now func() time.Time
}

type Server struct {
	o   Options
	mcp *mcp.Server
}

func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.ReminderEvery <= 0 {
		o.ReminderEvery = DefaultReminderEvery
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	s := &Server{o: o}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: "scrapbook01", Title: "Scrapbook/01", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: instructions,
		Logger:       o.Logger,
	})
	s.mcp.AddReceivingMiddleware(traceMethods)
	s.addTools()
	return s
}

// traceMethods gives each MCP method its own Sentry transaction name and
// span ("MCP tools/call search"), so tools get their own latency numbers.
func traceMethods(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		name := "MCP " + method
		if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && p != nil {
			name += " " + p.Name
		}
		telemetry.NameTransaction(ctx, name)
		ctx, span := telemetry.StartSpan(ctx, "mcp.server", name)
		res, err := next(ctx, method, req)
		if err == nil {
			if r, ok := res.(*mcp.CallToolResult); ok && r != nil && r.IsError {
				span.SetData("mcp.tool.error", true)
			}
		}
		span.End(err)
		return res, err
	}
}

// Handler serves the MCP endpoint and its OAuth metadata. Mount it at "/".
func (s *Server) Handler() http.Handler {
	resource := strings.TrimRight(s.o.PublicURL, "/") + Path
	var servers []string // empty in local dev without WorkOS
	if s.o.AuthServer != "" {
		servers = []string{s.o.AuthServer}
	}
	meta := sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   servers,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Scrapbook/01",
	})

	verify := func(_ context.Context, token string, r *http.Request) (*sdkauth.TokenInfo, error) {
		id, exp, err := s.o.Verifier.Verify(token)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
		}
		if r != nil {
			telemetry.SetUser(r.Context(), id.String())
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
	ID       string   `json:"id"`
	Title    string   `json:"title" jsonschema:"what the user said the item is about, or its site when undescribed"`
	Category string   `json:"category,omitempty"`
	URL      string   `json:"url"`
	Platform string   `json:"platform"`
	SavedOn  string   `json:"savedOn" jsonschema:"date saved, YYYY-MM-DD"`
	Score    float64  `json:"score,omitempty" jsonschema:"similarity to the query, 0 to 1 (semantic search only)"`
	Summary  string   `json:"summary,omitempty" jsonschema:"the item's analysis summary, when it has one"`
	UseWhen  []string `json:"useWhen,omitempty" jsonschema:"situations in which the user wants this item brought up, from its analysis"`
}

// Analysis is what the user's assistant learned about an item's content.
type Analysis struct {
	Summary   string    `json:"summary" jsonschema:"the central idea in two or three sentences"`
	Details   string    `json:"details,omitempty" jsonschema:"the concrete content: lists, steps, places, prices, names, links"`
	Tags      []string  `json:"tags,omitempty" jsonschema:"short topic tags, lowercase, up to 20"`
	UseWhen   []string  `json:"useWhen,omitempty" jsonschema:"situations in which to bring this item up unprompted, e.g. 'planning a trip to Buenos Aires', up to 10"`
	Actions   []string  `json:"actions,omitempty" jsonschema:"things the user could do with it, up to 10"`
	Caveats   string    `json:"caveats,omitempty" jsonschema:"what to keep in mind: sponsored content, missing audio, outdated information"`
	Sources   []string  `json:"sources,omitempty" jsonschema:"what the analysis was made from, e.g. caption, video_frames, transcript, image_text"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
}

type AnalysisInput struct {
	ID string `json:"id" jsonschema:"item id from search_library"`
	Analysis
}

type AnalysisOutput struct {
	ID      string `json:"id"`
	Created bool   `json:"created" jsonschema:"false if the item already had an analysis (it was replaced)"`
}

type SearchInput struct {
	Query    string `json:"query" jsonschema:"what to look for, in the user's words, e.g. 'pasta recipes' or 'things to do in Montevideo'"`
	Category string `json:"category,omitempty" jsonschema:"only search this category (exact name)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max results, 1-10, default 5"`
}

type SearchOutput struct {
	Results  []Result `json:"results"`
	Mode     string   `json:"mode" jsonschema:"semantic, semantic+rerank or keyword"`
	Reminder string   `json:"reminder,omitempty" jsonschema:"a note to pass on to the user once per conversation"`
}

type ItemInput struct {
	ID string `json:"id" jsonschema:"item id from search_library"`
}

type ItemOutput struct {
	Result
	Description *string   `json:"description"`
	Analysis    *Analysis `json:"analysis,omitempty" jsonschema:"what is known about the item's content, when it has been analyzed"`
	Related     []Result  `json:"related" jsonschema:"items most similar to this one"`
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
		Name: "search_library",
		Description: "Search the user's saved posts and links by meaning. Call it on your own whenever the user is planning, " +
			"deciding, creating, buying or learning something their saves could help with, not only when they mention a save. " +
			"Results include each save's summary and useWhen situations; use get_item for details.",
		Annotations: readOnly,
	}, s.search)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_item",
		Description: "Get one saved item with its full note, its analysis if it has one, and the items most similar to it.",
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

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "save_analysis",
		Description: "Record what a saved item contains: a summary, the concrete details, tags, the situations it's useful in and " +
			"anything to keep in mind. Write it from content the user showed you or that you opened yourself; never guess " +
			"from the link alone. Replaces the item's previous analysis. The user's own note stays as it is.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &f, IdempotentHint: true},
	}, s.saveAnalysis)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "get_analysis_queue",
		Description: "List the saves that need an analysis (never analyzed, or saved again since), with when the user " +
			"last analyzed their library and whether a reminder is due. Use it to work through their saves with save_analysis.",
		Annotations: readOnly,
	}, s.analysisQueue)
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
			// With a re-ranker, fetch extra candidates for it to choose from.
			fetch := limit
			if s.o.Rerank != nil {
				fetch = min(limit*2, 10)
			}
			rows, err := s.o.Queries.SearchItemsByVector(ctx, db.SearchItemsByVectorParams{
				UserID: user, Embedding: ai.VectorLiteral(vecs[0]), Category: category, Lim: int32(fetch),
			})
			if err != nil {
				return nil, out, err
			}
			out.Mode = "semantic"
			for _, r := range rows {
				res := result(r.ID, r.Url, r.Platform, r.Description, r.CategoryName, r.CreatedAt, r.Score)
				res.Summary, res.UseWhen = deref(r.AnalysisSummary), r.AnalysisUseWhen
				out.Results = append(out.Results, res)
			}
			if s.o.Rerank != nil && len(out.Results) > 1 {
				if reordered, err := s.rerank(ctx, q, out.Results); err == nil {
					out.Results, out.Mode = reordered, "semantic+rerank"
				} else {
					s.o.Logger.WarnContext(ctx, "mcp search: rerank failed, keeping vector order", "err", err)
				}
			}
			if len(out.Results) > limit {
				out.Results = out.Results[:limit]
			}
			out.Reminder = s.reminderFor(ctx, user)
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
		res := result(r.ID, r.Url, r.Platform, r.Description, r.CategoryName, r.CreatedAt, 0)
		res.Summary, res.UseWhen = deref(r.AnalysisSummary), r.AnalysisUseWhen
		out.Results = append(out.Results, res)
	}
	out.Reminder = s.reminderFor(ctx, user)
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
	a, err := s.o.Queries.GetItemAnalysis(ctx, db.GetItemAnalysisParams{ItemID: id, UserID: user})
	switch {
	case err == nil:
		out.Analysis = &Analysis{
			Summary: a.Summary, Details: deref(a.Details), Tags: a.Tags, UseWhen: a.UseWhen,
			Actions: a.Actions, Caveats: deref(a.Caveats), Sources: a.Sources, UpdatedAt: a.UpdatedAt,
		}
		out.Summary = a.Summary
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, out, err
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
	if row.Inserted && s.o.Enqueuer != nil {
		if err := s.o.Enqueuer.EnqueuePreview(ctx, row.ID); err != nil {
			s.o.Logger.ErrorContext(ctx, "mcp save: enqueue preview", "err", err)
		}
	}
	return nil, SaveOutput{ID: row.ID.String(), URL: norm, Created: row.Inserted}, nil
}

// Limits for save_analysis, matching the table's checks.
const (
	maxSummary = 1000
	maxDetails = 8000
	maxCaveats = 1000
	maxTags    = 20
	maxList    = 10
	maxEntry   = 200 // runes per tag, situation, action or source
)

func (s *Server) saveAnalysis(ctx context.Context, req *mcp.CallToolRequest, in AnalysisInput) (*mcp.CallToolResult, AnalysisOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, AnalysisOutput{}, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, AnalysisOutput{}, errors.New("unknown item id")
	}
	summary := clip(strings.TrimSpace(in.Summary), maxSummary)
	if summary == "" {
		return nil, AnalysisOutput{}, errors.New("summary is required")
	}
	// Check ownership before spending anything on an embedding.
	if _, err := s.o.Queries.GetItem(ctx, db.GetItemParams{ID: id, UserID: user}); errors.Is(err, pgx.ErrNoRows) {
		return nil, AnalysisOutput{}, errors.New("unknown item id")
	} else if err != nil {
		return nil, AnalysisOutput{}, err
	}
	tags := cleanList(in.Tags, maxTags, true)
	useWhen := cleanList(in.UseWhen, maxList, false)
	params := db.UpsertItemAnalysisParams{
		ItemID: id, UserID: user, Summary: summary,
		Details: optional(in.Details, maxDetails), Caveats: optional(in.Caveats, maxCaveats),
		Tags: tags, UseWhen: useWhen, Actions: cleanList(in.Actions, maxList, false),
		Sources: cleanList(in.Sources, maxList, true),
	}

	// One small embedding of what the item is about, charged like a search.
	// Without embeddings or credits the analysis is still saved and found by
	// keyword search.
	embed := s.o.Embed != nil
	if embed && s.o.Credits != nil {
		if embed, err = s.o.Credits.CanSpend(ctx, user); err != nil {
			return nil, AnalysisOutput{}, err
		}
		if embed {
			var meter *ai.Meter
			ctx, meter = ai.WithMeter(ctx)
			defer func() {
				if cerr := s.o.Credits.Charge(context.WithoutCancel(ctx), user, "analysis", &id, meter); cerr != nil {
					s.o.Logger.ErrorContext(ctx, "mcp save_analysis: charge credits", "err", cerr)
				}
			}()
		}
	}
	if embed {
		if vecs, err := s.o.Embed.Embed(ctx, analysisText(summary, tags, useWhen)); err == nil {
			v := ai.VectorLiteral(vecs[0])
			params.Embedding = &v
		} else {
			s.o.Logger.WarnContext(ctx, "mcp save_analysis: embedding failed, saving without it", "err", err)
		}
	}

	row, err := s.o.Queries.UpsertItemAnalysis(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, AnalysisOutput{}, errors.New("unknown item id")
	}
	if err != nil {
		return nil, AnalysisOutput{}, err
	}
	return nil, AnalysisOutput{ID: row.ItemID.String(), Created: row.Inserted}, nil
}

// analysisText is what gets embedded for an analysis: what the item is about
// and when it's useful. The long details stay out to keep the input small.
func analysisText(summary string, tags, useWhen []string) string {
	parts := []string{summary}
	if len(tags) > 0 {
		parts = append(parts, strings.Join(tags, ", "))
	}
	if len(useWhen) > 0 {
		parts = append(parts, strings.Join(useWhen, "; "))
	}
	return strings.Join(parts, "\n")
}

// cleanList trims entries, drops empty ones and duplicates (ignoring case),
// clips each entry and keeps at most limit of them.
func cleanList(in []string, limit int, lower bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, e := range in {
		e = clip(strings.TrimSpace(e), maxEntry)
		if lower {
			e = strings.ToLower(e)
		}
		k := strings.ToLower(e)
		if e == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out
}

func optional(s string, limit int) *string {
	if s = clip(strings.TrimSpace(s), limit); s == "" {
		return nil
	}
	return &s
}

func clip(s string, limit int) string {
	if r := []rune(s); len(r) > limit {
		return strings.TrimSpace(string(r[:limit]))
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

const rerankInstructions = "Which saved item best matches what the user is looking for?"

// rerank asks Jev to choose among the candidates and orders them by its
// probabilities (ties keep the vector order).
func (s *Server) rerank(ctx context.Context, query string, cands []Result) ([]Result, error) {
	options := make(map[string]string, len(cands))
	for i, c := range cands {
		desc := c.Title
		if c.Category != "" {
			desc += " (" + c.Category + ")"
		}
		if c.Summary != "" {
			desc += ": " + c.Summary
		}
		options[strconv.Itoa(i+1)] = desc
	}
	choice, err := s.o.Rerank.Choose(ctx, query, rerankInstructions, options)
	if err != nil {
		return nil, err
	}
	prob := func(i int) float64 {
		label := strconv.Itoa(i + 1)
		if p, ok := choice.Probabilities[label]; ok {
			return p
		}
		if label == choice.Label {
			return choice.Confidence
		}
		return 0
	}
	idx := make([]int, len(cands))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return prob(idx[a]) > prob(idx[b]) })
	out := make([]Result, len(cands))
	for i, j := range idx {
		out[i] = cands[j]
	}
	return out, nil
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
