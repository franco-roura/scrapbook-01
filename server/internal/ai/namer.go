package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Namer asks Claude Haiku for a short category name. It only runs when Jev
// finds no existing category that fits, so most saves never reach it.
type Namer struct {
	client anthropic.Client
	model  string
	// Price per million tokens, for metering. Claude Haiku 5.5 lists $0.10
	// in / $0.50 out (prompts up to 100K tokens). Thinking tokens are
	// billed as output and are included in the usage the API reports.
	Price Price
}

// NewNamer builds a Namer. Extra options (base URL, retries) are mainly for
// tests; the API key comes from the argument, or ANTHROPIC_API_KEY when empty.
func NewNamer(apiKey, model string, opts ...option.RequestOption) *Namer {
	if apiKey != "" {
		opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	}
	return &Namer{client: anthropic.NewClient(opts...), model: model}
}

const namerSystem = `You name categories in a personal scrapbook of saved social media posts and links.
Given what the user said a saved item is about, reply with one category name for it:
- 1 to 3 words, sentence case, no punctuation, no emoji, no quotes.
- Broad enough that similar items will fit later ("Recipes", not "Pasta with lemon").
- Write it in the same language as the item's description.
- Different from every existing category listed: the item didn't fit any of them.
Reply with the name only.`

// ErrRefused means retrying won't help: the model declined, or the API
// rejected the request (for example a missing or invalid key).
var ErrRefused = errors.New("category namer can't answer")

// Name proposes a category name for description, avoiding the existing ones.
func (n *Namer) Name(ctx context.Context, description string, existing []string) (string, error) {
	var prompt strings.Builder
	prompt.WriteString("Item description:\n")
	prompt.WriteString(description)
	prompt.WriteString("\n\nExisting categories: ")
	if len(existing) == 0 {
		prompt.WriteString("(none yet)")
	} else {
		prompt.WriteString(strings.Join(existing, ", "))
	}

	resp, err := n.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model: n.model,
		// Thinking is on by default on Haiku 5.5 and counts toward this cap;
		// low effort keeps it short. The answer itself is a few tokens.
		MaxTokens:    1024,
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		System:       []anthropic.TextBlockParam{{Text: namerSystem}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt.String())),
		},
	})
	if err != nil {
		// Client errors (bad key, bad request) won't fix themselves on retry.
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != 429 {
			return "", fmt.Errorf("%w: %v", ErrRefused, err)
		}
		return "", fmt.Errorf("claude: %w", err)
	}
	in, out := resp.Usage.InputTokens, resp.Usage.OutputTokens
	record(ctx, Usage{Service: "claude", Model: n.model, InputTokens: in, OutputTokens: out, Cost: n.Price.cost(in, out)})
	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", ErrRefused
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	name := CleanCategoryName(text.String())
	if name == "" {
		return "", fmt.Errorf("claude: empty category name (stop reason %s)", resp.StopReason)
	}
	return name, nil
}

// CleanCategoryName keeps the first line, strips quotes and trailing
// punctuation, collapses spaces and caps the length at 40 characters.
func CleanCategoryName(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '&' || unicode.IsSymbol(r)
	})
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 40 {
		s = strings.TrimSpace(string([]rune(s)[:40]))
	}
	return s
}
