package mcpserver

import (
	"slices"
	"strings"
	"testing"
)

func TestCleanList(t *testing.T) {
	got := cleanList([]string{"  Recipes ", "recipes", "", "Weeknight Dinners", strings.Repeat("x", 300)}, 3, true)
	want := []string{"recipes", "weeknight dinners", strings.Repeat("x", maxEntry)}
	if !slices.Equal(got, want) {
		t.Fatalf("cleanList = %q, want %q", got, want)
	}

	// Situations keep their case; the limit applies after dropping duplicates.
	got = cleanList([]string{"Planning a trip", "planning a trip", "Cooking tonight", "Hosting friends"}, 2, false)
	if !slices.Equal(got, []string{"Planning a trip", "Cooking tonight"}) {
		t.Fatalf("cleanList kept %q", got)
	}

	if got := cleanList(nil, 5, true); got == nil || len(got) != 0 {
		t.Fatalf("cleanList(nil) = %#v, want an empty, non-nil slice", got)
	}
}

func TestOptionalAndClip(t *testing.T) {
	if optional("   ", 10) != nil {
		t.Fatal("blank text should be stored as NULL")
	}
	if got := *optional(" ñandú tour ", 5); got != "ñandú" {
		t.Fatalf("clip counts runes: got %q", got)
	}
}

func TestAnalysisText(t *testing.T) {
	got := analysisText("Five pasta recipes under 20 minutes.", []string{"recipes", "pasta"}, []string{"cooking tonight", "short on time"})
	want := "Five pasta recipes under 20 minutes.\nrecipes, pasta\ncooking tonight; short on time"
	if got != want {
		t.Fatalf("analysisText = %q, want %q", got, want)
	}
	if got := analysisText("Just a summary.", nil, nil); got != "Just a summary." {
		t.Fatalf("analysisText without lists = %q", got)
	}
}
