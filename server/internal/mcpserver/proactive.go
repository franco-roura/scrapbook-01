package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

// instructions tell the assistant how to use the library. They're sent once,
// when the client connects, and they're the main lever for making it bring
// saves up on its own instead of waiting to be asked.
const instructions = `Scrapbook/01 is the user's library of posts and links they saved from social media and the web to use later. Each save has the user's short note and a category; many also have an analysis: a summary, details, tags, actions, caveats and useWhen, the situations in which the user wants it brought up. Your job is to bring saves back at the moment they're useful, without being asked.

Be proactive:
- Whenever the user is planning, deciding, creating, buying, learning or fixing something (a trip, a meal, a purchase, a campaign, a workout, a project, a gift, a problem at work), call search_library before you answer, even if they never mention their saves. Search for the situation in their words, e.g. "weekend trip to the coast", "writing hooks for a product launch". Search again when the topic changes.
- Results can carry useWhen. When the conversation matches one of those situations, mention the save; that is exactly what the user asked for when they saved it.
- Bring up at most three saves at a time, one line each: what it is, why it fits right now, and its link. Before relying on a save, open it with get_item for its details, actions and caveats. Prefer the user's saves over general knowledge, and say when an idea comes from them.
- Turn actions into concrete next steps. Respect caveats: say when a save is sponsored, unverified or possibly outdated.
- Don't repeat a save you already brought up in this conversation, and don't search for small talk.

Keep the library rich:
- When a tool result carries a reminder, pass it on once per conversation, in one or two sentences, and offer to do the analysis now.
- To analyze, call get_analysis_queue and go through the saves it returns. Read each save's content when you can: open the link if you have a browser, or ask the user to paste the caption or share screenshots. Record what you learned with save_analysis. Only write what you actually saw, and put in caveats what you couldn't check (for example, audio you couldn't hear).
- useWhen drives everything above, so make it specific: three to six situations written the way the user would describe what they're doing, e.g. "planning a trip to Lisbon", "choosing a project management tool", "cooking dinner for friends". Vague situations like "learning" or "inspiration" never match anything.
- Whenever the user shows you what a saved item contains, record it with save_analysis.`

// DefaultReminderEvery is how often the user is reminded to analyze new saves.
const DefaultReminderEvery = 15 * 24 * time.Hour

const queueDefault, queueMax = 10, 25

type QueueInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"max saves to return, 1-25, default 10"`
}

type QueueItem struct {
	Result
	AnalyzedOn string `json:"analyzedOn,omitempty" jsonschema:"date of the previous analysis, YYYY-MM-DD, when the save was saved again since"`
}

type QueueOutput struct {
	Status
	Items []QueueItem `json:"items" jsonschema:"saves to analyze: never-analyzed ones first, newest first"`
}

// Status says when the library was last analyzed and whether a reminder is due.
type Status struct {
	LastAnalysisOn string `json:"lastAnalysisOn,omitempty" jsonschema:"date of the most recent analysis, YYYY-MM-DD; empty if the user never analyzed a save"`
	DaysSince      *int   `json:"daysSinceLastAnalysis,omitempty"`
	Total          int    `json:"total" jsonschema:"saves in the library"`
	Pending        int    `json:"pending" jsonschema:"saves with no analysis, or saved again since their analysis"`
	Due            bool   `json:"due" jsonschema:"true when it's time to remind the user to analyze their saves"`
}

// status reads the user's analysis status. A reminder is due when some saves
// need an analysis and the last analysis is older than ReminderEvery (or
// there was none).
func (s *Server) status(ctx context.Context, user uuid.UUID) (Status, error) {
	row, err := s.o.Queries.GetAnalysisStatus(ctx, user)
	if err != nil {
		return Status{}, err
	}
	st := Status{Total: int(row.Total), Pending: int(row.Pending)}
	stale := true
	if row.Analyzed > 0 {
		st.LastAnalysisOn = row.LastAnalysisAt.UTC().Format("2006-01-02")
		age := s.o.Now().Sub(row.LastAnalysisAt)
		days := int(age / (24 * time.Hour))
		st.DaysSince = &days
		stale = age >= s.o.ReminderEvery
	}
	st.Due = st.Pending > 0 && stale
	return st, nil
}

// reminder is the note added to tool results when an analysis is due. It's
// written for the assistant, which passes it on in its own words.
func (st Status) reminder() string {
	if !st.Due {
		return ""
	}
	saves := "saves need"
	if st.Pending == 1 {
		saves = "save needs"
	}
	if st.DaysSince == nil {
		return fmt.Sprintf("Reminder for the user: %d %s an analysis, and they have never analyzed their library. "+
			"Analyzed saves are the ones you can bring up at the right moment. Once in this conversation, offer to "+
			"analyze them now with get_analysis_queue and save_analysis.", st.Pending, saves)
	}
	return fmt.Sprintf("Reminder for the user: their last analysis was %d days ago (%s) and %d %s one. "+
		"Once in this conversation, offer to analyze them now with get_analysis_queue and save_analysis.",
		*st.DaysSince, st.LastAnalysisOn, st.Pending, saves)
}

// reminderFor returns the reminder to attach to a tool result, if one is due.
// A failure only costs the reminder, never the tool call.
func (s *Server) reminderFor(ctx context.Context, user uuid.UUID) string {
	st, err := s.status(ctx, user)
	if err != nil {
		s.o.Logger.WarnContext(ctx, "mcp: analysis status", "err", err)
		return ""
	}
	return st.reminder()
}

func (s *Server) analysisQueue(ctx context.Context, req *mcp.CallToolRequest, in QueueInput) (*mcp.CallToolResult, QueueOutput, error) {
	user, err := userFrom(req)
	if err != nil {
		return nil, QueueOutput{}, err
	}
	limit := in.Limit
	if limit < 1 || limit > queueMax {
		limit = queueDefault
	}
	st, err := s.status(ctx, user)
	if err != nil {
		return nil, QueueOutput{}, err
	}
	rows, err := s.o.Queries.ListItemsToAnalyze(ctx, db.ListItemsToAnalyzeParams{UserID: user, Lim: int32(limit)})
	if err != nil {
		return nil, QueueOutput{}, err
	}
	out := QueueOutput{Status: st, Items: []QueueItem{}}
	for _, r := range rows {
		it := QueueItem{Result: result(r.ID, r.Url, r.Platform, r.Description, r.CategoryName, r.CreatedAt, 0)}
		if r.AnalyzedAt != nil {
			it.AnalyzedOn = r.AnalyzedAt.UTC().Format("2006-01-02")
		}
		out.Items = append(out.Items, it)
	}
	return nil, out, nil
}
