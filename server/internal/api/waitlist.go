package api

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/aaugoustis/waitlist/server/internal/waitlist"
)

// Waitlist adds someone to the sign-up waitlist. waitlist.WorkOS implements it.
type Waitlist interface {
	Join(ctx context.Context, email, name string) (state string, err error)
}

type joinWaitlistRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// joinWaitlist is POST /v1/waitlist, the landing page's form. It needs no
// sign-in. WorkOS keeps one entry per email, so repeats are harmless.
func (s *Server) joinWaitlist(w http.ResponseWriter, r *http.Request) {
	if s.waitlist == nil {
		writeError(w, http.StatusServiceUnavailable, "waitlist_off", "La lista de espera no está abierta todavía.")
		return
	}
	var req joinWaitlistRequest
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_name", "Escribí tu nombre.")
		return
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") || len(email) > 254 {
		writeError(w, http.StatusBadRequest, "invalid_email", "Revisá tu email.")
		return
	}
	state, err := s.waitlist.Join(r.Context(), email, name)
	if errors.Is(err, waitlist.ErrAlreadyUser) {
		writeError(w, http.StatusConflict, "already_user", "Ya tenés cuenta: entrá desde la app.")
		return
	}
	if err != nil {
		s.log.ErrorContext(r.Context(), "waitlist join failed", "err", err)
		writeError(w, http.StatusBadGateway, "waitlist_failed", "No pudimos anotarte. Probá de nuevo en un minuto.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state})
}
