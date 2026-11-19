// Package push sends notifications through Expo's push service, which
// forwards them to APNs and FCM. Docs: https://docs.expo.dev/push-notifications/sending-notifications/
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/franco-roura/scrapbook-01/server/internal/db"
	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

const defaultEndpoint = "https://exp.host/--/api/v2/push/send"

// Expo sends "Guardado en <category>" notifications to a user's devices.
type Expo struct {
	Q           db.Querier
	AccessToken string // optional; required if the Expo project enforces push security
	Endpoint    string // defaults to Expo's production endpoint
	HTTP        *http.Client
}

type message struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data"`
	Sound string            `json:"sound,omitempty"`
}

type ticket struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

// CategoryAssigned implements classify.Notifier.
func (e *Expo) CategoryAssigned(ctx context.Context, userID, itemID uuid.UUID, category string) error {
	tokens, err := e.Q.ListDeviceTokens(ctx, userID)
	if err != nil || len(tokens) == 0 {
		return err
	}
	msgs := make([]message, len(tokens))
	for i, t := range tokens {
		msgs[i] = message{
			To:    t,
			Title: "Guardado en " + category,
			Body:  "Tocá para cambiar la categoría.",
			Data:  map[string]string{"itemId": itemID.String(), "url": "scrapbook01://item/" + itemID.String()},
		}
	}
	tickets, err := e.send(ctx, msgs)
	if err != nil {
		return err
	}
	// Forget tokens for apps that were uninstalled or signed out.
	var gone []string
	for i, t := range tickets {
		if i < len(tokens) && t.Status == "error" && t.Details.Error == "DeviceNotRegistered" {
			gone = append(gone, tokens[i])
		}
	}
	if len(gone) > 0 {
		return e.Q.DeleteDeviceTokens(ctx, gone)
	}
	return nil
}

func (e *Expo) send(ctx context.Context, msgs []message) ([]ticket, error) {
	body, err := json.Marshal(msgs)
	if err != nil {
		return nil, err
	}
	endpoint := e.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	c := e.HTTP
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second, Transport: telemetry.Transport(nil)}
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("expo push: HTTP %d: %.200s", res.StatusCode, data)
	}
	var out struct {
		Data []ticket `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("expo push: decode: %w", err)
	}
	return out.Data, nil
}
