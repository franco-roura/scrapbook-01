package waitlist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJoin(t *testing.T) {
	var got entryRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/user_management/waitlists/default/entries" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk_test" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"waitlist_entry","state":"pending"}`))
	}))
	defer srv.Close()

	w := &WorkOS{APIKey: "sk_test", BaseURL: srv.URL}
	state, err := w.Join(context.Background(), "ana@example.com", "Ana")
	if err != nil || state != "pending" {
		t.Fatalf("Join = %q, %v", state, err)
	}
	if got.Email != "ana@example.com" || got.AdditionalFields["name"] != "Ana" || !got.SendConfirmationEmail {
		t.Errorf("body %+v", got)
	}
}

func TestJoinErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusUnprocessableEntity, `{"code":"user_already_exists","message":"A user with this email already exists."}`, ErrAlreadyUser},
		{http.StatusUnauthorized, `{"message":"Unauthorized"}`, nil},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := (&WorkOS{APIKey: "sk_test", BaseURL: srv.URL}).Join(context.Background(), "ana@example.com", "Ana")
		srv.Close()
		if err == nil {
			t.Errorf("%d: no error", tc.status)
		} else if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("%d: got %v, want %v", tc.status, err, tc.want)
		}
	}
}
