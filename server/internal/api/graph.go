package api

import (
	"net/http"

	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/graph"
)

const (
	// graphItemLimit caps the global graph; the view stays smooth on phones
	// up to a few thousand nodes.
	graphItemLimit = 2000
	// neighborLimit caps the local graph around one item.
	neighborLimit = 60
)

// loadGraph builds the user's whole graph from the newest items.
func (s *Server) loadGraph(w http.ResponseWriter, r *http.Request) (graph.Graph, bool) {
	user, _ := auth.UserFrom(r.Context())
	items, err := s.q.ListGraphItems(r.Context(), db.ListGraphItemsParams{UserID: user, Lim: graphItemLimit + 1})
	if err != nil {
		s.internal(w, r, "list graph items", err)
		return graph.Graph{}, false
	}
	truncated := len(items) > graphItemLimit
	if truncated {
		items = items[:graphItemLimit]
	}
	edges, err := s.q.ListEdgesForUser(r.Context(), user)
	if err != nil {
		s.internal(w, r, "list edges", err)
		return graph.Graph{}, false
	}
	return graph.Build(items, edges, truncated), true
}

// getGraph serves GET /v1/graph: every item, category hub and link.
func (s *Server) getGraph(w http.ResponseWriter, r *http.Request) {
	g, ok := s.loadGraph(w, r)
	if ok {
		writeJSON(w, http.StatusOK, g)
	}
}

// getNeighbors serves GET /v1/items/{id}/neighbors?depth=1|2: the local
// graph the item screen draws.
func (s *Server) getNeighbors(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	depth := 1
	switch r.URL.Query().Get("depth") {
	case "", "1":
	case "2":
		depth = 2
	default:
		writeError(w, http.StatusBadRequest, "bad_depth", "depth must be 1 or 2.")
		return
	}
	full, ok := s.loadGraph(w, r)
	if !ok {
		return
	}
	local, found := graph.Neighborhood(full, id.String(), depth, neighborLimit)
	if !found {
		// Older than the graph's item limit, or not the user's item.
		user, _ := auth.UserFrom(r.Context())
		it, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Item not found.")
			return
		}
		local = graph.Graph{Links: []graph.Link{}, Nodes: []graph.Node{{
			ID: it.ID.String(), Kind: "item", Label: graph.Label(it.Description, it.Url), Platform: it.Platform,
		}}}
	}
	writeJSON(w, http.StatusOK, local)
}
