package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

// statusOnly is a Querier that only answers GetAnalysisStatus.
type statusOnly struct {
	db.Querier
	row db.GetAnalysisStatusRow
}

func (q statusOnly) GetAnalysisStatus(context.Context, uuid.UUID) (db.GetAnalysisStatusRow, error) {
	return q.row, nil
}

func TestStatusAndReminder(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	day := 24 * time.Hour

	for _, tc := range []struct {
		name    string
		row     db.GetAnalysisStatusRow
		due     bool
		days    int // -1: never analyzed
		mention string
	}{
		{"never analyzed, saves waiting", db.GetAnalysisStatusRow{Total: 4, Pending: 4}, true, -1, "never analyzed"},
		{"never analyzed, empty library", db.GetAnalysisStatusRow{}, false, -1, ""},
		{"analyzed recently", db.GetAnalysisStatusRow{LastAnalysisAt: ago(3 * day), Analyzed: 2, Total: 5, Pending: 3}, false, 3, ""},
		{"15 days ago, new saves", db.GetAnalysisStatusRow{LastAnalysisAt: ago(15 * day), Analyzed: 2, Total: 3, Pending: 1}, true, 15, "15 days ago (2026-11-07) and 1 save needs one"},
		{"long ago, nothing pending", db.GetAnalysisStatusRow{LastAnalysisAt: ago(40 * day), Analyzed: 3, Total: 3}, false, 40, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(Options{Queries: statusOnly{row: tc.row}, Now: func() time.Time { return now }})
			st, err := s.status(context.Background(), uuid.New())
			if err != nil {
				t.Fatal(err)
			}
			if st.Due != tc.due {
				t.Fatalf("due = %v, want %v (%+v)", st.Due, tc.due, st)
			}
			switch {
			case tc.days < 0 && (st.DaysSince != nil || st.LastAnalysisOn != ""):
				t.Fatalf("never analyzed, but got %+v", st)
			case tc.days >= 0 && (st.DaysSince == nil || *st.DaysSince != tc.days):
				t.Fatalf("days since = %v, want %d", st.DaysSince, tc.days)
			}
			r := st.reminder()
			if (r != "") != tc.due {
				t.Fatalf("reminder %q while due = %v", r, tc.due)
			}
			if tc.mention != "" && !strings.Contains(r, tc.mention) {
				t.Fatalf("reminder %q doesn't mention %q", r, tc.mention)
			}
		})
	}
}

func TestReminderEveryIsConfigurable(t *testing.T) {
	now := time.Now()
	row := db.GetAnalysisStatusRow{LastAnalysisAt: now.Add(-8 * 24 * time.Hour), Analyzed: 1, Total: 2, Pending: 1}
	s := New(Options{Queries: statusOnly{row: row}, ReminderEvery: 7 * 24 * time.Hour, Now: func() time.Time { return now }})
	st, err := s.status(context.Background(), uuid.New())
	if err != nil || !st.Due {
		t.Fatalf("with a 7-day reminder, 8 days should be due: %+v %v", st, err)
	}
	if New(Options{}).o.ReminderEvery != 15*24*time.Hour {
		t.Fatal("the default reminder interval should be 15 days")
	}
}

func TestInstructionsCoverTheTools(t *testing.T) {
	for _, tool := range []string{"search_library", "get_item", "save_analysis", "get_analysis_queue", "useWhen", "reminder"} {
		if !strings.Contains(instructions, tool) {
			t.Errorf("instructions don't mention %s", tool)
		}
	}
}
