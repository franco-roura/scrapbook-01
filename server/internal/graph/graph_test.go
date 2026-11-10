package graph

import (
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/db"
)

func ptr[T any](v T) *T { return &v }

// A chain a - b - c - d of similar items, plus e alone; a and b are in
// "Parenting", c in "Crafts".
func fixture() (Graph, map[string]uuid.UUID) {
	ids := map[string]uuid.UUID{}
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		ids[k] = uuid.New()
	}
	parenting, crafts := uuid.New(), uuid.New()
	items := []db.ListGraphItemsRow{
		{ID: ids["a"], Url: "https://instagram.com/p/1", Description: ptr("traditions with my daughter"), CategoryID: &parenting, CategoryName: ptr("Parenting")},
		{ID: ids["b"], Url: "https://tiktok.com/x", Description: ptr("weekend with my daughter"), CategoryID: &parenting, CategoryName: ptr("Parenting")},
		{ID: ids["c"], Url: "https://youtube.com/watch?v=1", Description: ptr("paper crafts"), CategoryID: &crafts, CategoryName: ptr("Crafts")},
		{ID: ids["d"], Url: "https://www.example.com/d"},
		{ID: ids["e"], Url: "https://example.com/e", Description: ptr(strings.Repeat("long ", 20))},
	}
	edges := []db.ListEdgesForUserRow{
		{Src: ids["a"], Dst: ids["b"], Weight: 0.9},
		{Src: ids["b"], Dst: ids["c"], Weight: 0.6},
		{Src: ids["c"], Dst: ids["d"], Weight: 0.5},
		{Src: ids["a"], Dst: uuid.New(), Weight: 0.7}, // to an item outside the limit: dropped
	}
	return Build(items, edges, false), ids
}

func nodeIDs(g Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		out = append(out, n.Label)
	}
	sort.Strings(out)
	return out
}

func TestBuild(t *testing.T) {
	g, ids := fixture()
	if len(g.Nodes) != 7 { // 5 items + 2 categories
		t.Fatalf("nodes = %d", len(g.Nodes))
	}
	var similar, category int
	for _, l := range g.Links {
		if l.Kind == "similar" {
			similar++
		} else {
			category++
		}
	}
	if similar != 3 || category != 3 {
		t.Fatalf("links: %d similar, %d category", similar, category)
	}
	for _, n := range g.Nodes {
		switch n.ID {
		case ids["d"].String():
			if n.Label != "example.com" || n.Degree != 1 {
				t.Errorf("d: %+v", n)
			}
		case ids["e"].String():
			if len([]rune(n.Label)) != 60 || !strings.HasSuffix(n.Label, "…") || n.Degree != 0 {
				t.Errorf("e: %+v", n)
			}
		case ids["b"].String():
			if n.Degree != 3 { // a, c, Parenting
				t.Errorf("b degree = %d", n.Degree)
			}
		}
	}
}

func TestNeighborhood(t *testing.T) {
	g, ids := fixture()

	one, ok := Neighborhood(g, ids["a"].String(), 1, 50)
	if !ok {
		t.Fatal("center not found")
	}
	if got := nodeIDs(one); strings.Join(got, "|") != "Parenting|traditions with my daughter|weekend with my daughter" {
		t.Fatalf("depth 1 = %v", got)
	}
	if one.Nodes[0].ID != ids["a"].String() {
		t.Fatal("center isn't first")
	}

	two, _ := Neighborhood(g, ids["a"].String(), 2, 50)
	if got := nodeIDs(two); len(got) != 5 { // a, b, c + Parenting, Crafts
		t.Fatalf("depth 2 = %v", got)
	}
	for _, n := range two.Nodes {
		if n.ID == ids["d"].String() {
			t.Fatal("d is three hops away")
		}
	}

	capped, _ := Neighborhood(g, ids["b"].String(), 2, 2)
	if !capped.Truncated {
		t.Fatal("cap not reported")
	}
	items := 0
	for _, n := range capped.Nodes {
		if n.Kind == "item" {
			items++
			if n.ID == ids["c"].String() {
				t.Fatal("cap should keep the strongest link (a, 0.9) over c (0.6)")
			}
		}
	}
	if items != 2 {
		t.Fatalf("capped items = %d", items)
	}

	if _, ok := Neighborhood(g, uuid.NewString(), 1, 10); ok {
		t.Fatal("unknown center found")
	}
}
