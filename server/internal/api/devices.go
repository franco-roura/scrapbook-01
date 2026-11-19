package api

import (
	"net/http"
	"strings"

	"github.com/franco-roura/scrapbook-01/server/internal/auth"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

type registerDeviceRequest struct {
	Platform string `json:"platform"`
}

// registerDevice stores an Expo push token (PUT /v1/devices/{token}) so the
// user hears where saves were filed.
func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	token := r.PathValue("token")
	if !validPushToken(token) {
		writeError(w, http.StatusUnprocessableEntity, "bad_token", "That isn't an Expo push token.")
		return
	}
	var req registerDeviceRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Platform != "ios" && req.Platform != "android" {
		writeError(w, http.StatusUnprocessableEntity, "bad_platform", `platform must be "ios" or "android".`)
		return
	}
	if err := s.q.UpsertDevice(r.Context(), db.UpsertDeviceParams{Token: token, UserID: user, Platform: req.Platform}); err != nil {
		s.internal(w, r, "register device", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unregisterDevice forgets a token, e.g. on sign-out.
func (s *Server) unregisterDevice(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	if err := s.q.DeleteDevice(r.Context(), db.DeleteDeviceParams{Token: r.PathValue("token"), UserID: user}); err != nil {
		s.internal(w, r, "unregister device", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Expo tokens look like ExponentPushToken[xxxxxxxx] (or ExpoPushToken[...]).
func validPushToken(t string) bool {
	if len(t) > 200 || !strings.HasSuffix(t, "]") {
		return false
	}
	return strings.HasPrefix(t, "ExponentPushToken[") || strings.HasPrefix(t, "ExpoPushToken[")
}
