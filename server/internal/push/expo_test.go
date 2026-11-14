package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/db"
)

type fakeDevices struct {
	db.Querier
	tokens  []string
	deleted []string
}

func (f *fakeDevices) ListDeviceTokens(context.Context, uuid.UUID) ([]string, error) {
	return f.tokens, nil
}

func (f *fakeDevices) DeleteDeviceTokens(_ context.Context, t []string) error {
	f.deleted = append(f.deleted, t...)
	return nil
}

func TestCategoryAssigned(t *testing.T) {
	var sent []message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		io.WriteString(w, `{"data":[{"status":"ok","id":"1"},
			{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}]}`)
	}))
	defer srv.Close()

	q := &fakeDevices{tokens: []string{"ExponentPushToken[a]", "ExponentPushToken[b]"}}
	e := &Expo{Q: q, AccessToken: "secret", Endpoint: srv.URL}
	item := uuid.New()
	if err := e.CategoryAssigned(context.Background(), uuid.New(), item, "Parenting"); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent[0].Title != "Guardado en Parenting" || sent[1].Data["itemId"] != item.String() {
		t.Fatalf("sent %+v", sent)
	}
	if len(q.deleted) != 1 || q.deleted[0] != "ExponentPushToken[b]" {
		t.Fatalf("deleted %v", q.deleted)
	}

	// No devices: nothing is sent.
	sent = nil
	if err := (&Expo{Q: &fakeDevices{}, Endpoint: srv.URL}).CategoryAssigned(context.Background(), uuid.New(), item, "X"); err != nil || sent != nil {
		t.Fatalf("no devices: sent %v err %v", sent, err)
	}
}
