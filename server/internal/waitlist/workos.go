// Package waitlist adds people to the WorkOS AuthKit waitlist. Sign-up is
// closed in AuthKit; people join here (from the landing page), and approving
// an entry in the WorkOS dashboard emails them an invitation.
// Docs: https://workos.com/docs/authkit/waitlist
package waitlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrAlreadyUser means the email already has an account: sign in instead.
var ErrAlreadyUser = errors.New("waitlist: email already has an account")

// WorkOS calls the waitlist API with the environment's secret key.
type WorkOS struct {
	APIKey  string // sk_..., never sent to browsers
	BaseURL string // defaults to https://api.workos.com
	HTTP    *http.Client
}

type entryRequest struct {
	Email                 string            `json:"email"`
	AdditionalFields      map[string]string `json:"additional_fields,omitempty"`
	SendConfirmationEmail bool              `json:"send_confirmation_email"`
}

type entryResponse struct {
	State string `json:"state"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Join adds email to the default waitlist and returns the entry's state
// ("pending", "approved" or "denied"). Joining twice returns the existing
// entry, and WorkOS sends the confirmation email only the first time.
func (w *WorkOS) Join(ctx context.Context, email, name string) (string, error) {
	req := entryRequest{Email: email, SendConfirmationEmail: true}
	if name != "" {
		req.AdditionalFields = map[string]string{"name": name}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	base := w.BaseURL
	if base == "" {
		base = "https://api.workos.com"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/user_management/waitlists/default/entries", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	hreq.Header.Set("Authorization", "Bearer "+w.APIKey)
	hreq.Header.Set("Content-Type", "application/json")
	client := w.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(hreq)
	if err != nil {
		return "", fmt.Errorf("waitlist: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode >= 300 {
		var e errorResponse
		_ = json.Unmarshal(raw, &e)
		if e.Code == "user_already_exists" || e.Error == "user_already_exists" {
			return "", ErrAlreadyUser
		}
		return "", fmt.Errorf("waitlist: WorkOS returned %d: %s %s", res.StatusCode, e.Code+e.Error, e.Message)
	}
	var entry entryResponse
	if err := json.Unmarshal(raw, &entry); err != nil {
		return "", fmt.Errorf("waitlist: decode entry: %w", err)
	}
	return entry.State, nil
}
