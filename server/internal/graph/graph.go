// Package graph shapes a user's items, categories and similarity edges into
// the node/link lists the app's graph view draws.
//
// Categories are hub nodes: every categorized item links to its category,
// so the graph has structure even before embeddings exist. "similar" links
// come from the edges table (nearest neighbours by embedding).
package graph

import (
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/db"
)

type Node struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // "item" or "category"
	Label      string `json:"label"`
	Platform   string `json:"platform,omitempty"`
	CategoryID string `json:"categoryId,omitempty"`
	// Degree counts the node's links in this graph; the view sizes nodes by it.
	Degree int `json:"degree"`
}

type Link struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Kind   string  `json:"kind"` // "similar" or "category"
	Weight float32 `json:"weight"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
	// Truncated is true when the item limit cut the graph short.
	Truncated bool `json:"truncated"`
}

const categoryPrefix = "c:"

// CategoryNodeID keeps category ids apart from item ids in one id space.
func CategoryNodeID(id uuid.UUID) string { return categoryPrefix + id.String() }

// Label is what the view prints next to an item: its description, or the
// link's host when it has none.
func Label(description *string, rawURL string) string {
	if description != nil {
		if r := []rune(*description); len(r) > 60 {
			return strings.TrimSpace(string(r[:59])) + "…"
		}
		return *description
	}
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return strings.TrimPrefix(u.Host, "www.")
	}
	return rawURL
}

// Build makes the whole graph. truncated says whether items were capped.
func Build(items []db.ListGraphItemsRow, edges []db.ListEdgesForUserRow, truncated bool) Graph {
	g := Graph{Nodes: []Node{}, Links: []Link{}, Truncated: truncated}
	index := map[string]int{}
	add := func(n Node) {
		if _, ok := index[n.ID]; !ok {
			index[n.ID] = len(g.Nodes)
			g.Nodes = append(g.Nodes, n)
		}
	}
	link := func(l Link) {
		g.Links = append(g.Links, l)
		g.Nodes[index[l.Source]].Degree++
		g.Nodes[index[l.Target]].Degree++
	}

	for _, it := range items {
		n := Node{ID: it.ID.String(), Kind: "item", Label: Label(it.Description, it.Url), Platform: it.Platform}
		if it.CategoryID != nil {
			n.CategoryID = it.CategoryID.String()
		}
		add(n)
	}
	for _, it := range items {
		if it.CategoryID == nil || it.CategoryName == nil {
			continue
		}
		cid := CategoryNodeID(*it.CategoryID)
		add(Node{ID: cid, Kind: "category", Label: *it.CategoryName})
		link(Link{Source: it.ID.String(), Target: cid, Kind: "category", Weight: 1})
	}
	for _, e := range edges {
		s, d := e.Src.String(), e.Dst.String()
		_, okS := index[s]
		_, okD := index[d]
		if okS && okD {
			link(Link{Source: s, Target: d, Kind: "similar", Weight: e.Weight})
		}
	}
	return g
}

// Neighborhood is the local graph around one item: items reachable through
// "similar" links within depth hops (strongest links first, at most
// maxItems), plus the category hubs of those items. Categories are not
// walked through, or one hop would pull in a whole category.
func Neighborhood(full Graph, center string, depth, maxItems int) (Graph, bool) {
	if _, ok := find(full, center); !ok {
		return Graph{}, false
	}
	adj := map[string][]Link{}
	for _, l := range full.Links {
		if l.Kind == "similar" {
			adj[l.Source] = append(adj[l.Source], l)
			adj[l.Target] = append(adj[l.Target], l)
		}
	}
	for k := range adj {
		sort.Slice(adj[k], func(i, j int) bool { return adj[k][i].Weight > adj[k][j].Weight })
	}

	keep := map[string]bool{center: true}
	frontier := []string{center}
	truncated := false
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, id := range frontier {
			for _, l := range adj[id] {
				other := l.Target
				if other == id {
					other = l.Source
				}
				if keep[other] {
					continue
				}
				if len(keep) >= maxItems {
					truncated = true
					break
				}
				keep[other] = true
				next = append(next, other)
			}
		}
		frontier = next
	}

	byID := map[string]Node{}
	for _, n := range full.Nodes {
		byID[n.ID] = n
	}
	out := Graph{Nodes: []Node{}, Links: []Link{}, Truncated: truncated}
	seen := map[string]int{}
	add := func(id string) {
		if _, ok := seen[id]; !ok {
			n := byID[id]
			n.Degree = 0
			seen[id] = len(out.Nodes)
			out.Nodes = append(out.Nodes, n)
		}
	}
	// Stable order: center first, then the rest as they appear in full.
	add(center)
	for _, n := range full.Nodes {
		if keep[n.ID] {
			add(n.ID)
		}
	}
	for _, l := range full.Links {
		switch {
		case l.Kind == "similar" && keep[l.Source] && keep[l.Target]:
		case l.Kind == "category" && keep[l.Source]:
			add(l.Target)
		default:
			continue
		}
		out.Links = append(out.Links, l)
		out.Nodes[seen[l.Source]].Degree++
		out.Nodes[seen[l.Target]].Degree++
	}
	return out, true
}

func find(g Graph, id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
