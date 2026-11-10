// Package api is the HTTP API the mobile app and share sheet call.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/db"
)

// maxBody caps request bodies; a save is a URL plus a short description.
const maxBody = 16 << 10

type Server struct {
	q       db.Querier
	log     *slog.Logger
	ping    func(context.Context) error
	authMW  func(http.Handler) http.Handler
	cors    map[string]bool
	enqueue Enqueuer
}

// Enqueuer queues background work. The jobs package implements it.
type Enqueuer interface {
	EnqueueClassify(ctx context.Context, itemID uuid.UUID) error
}

type Options struct {
	Queries  db.Querier
	Logger   *slog.Logger
	Verifier *auth.Verifier
	// Ping checks the database for /healthz. Optional.
	Ping func(context.Context) error
	// CORSOrigins are browser origins allowed to call the API (the web
	// build of the app). Native apps don't need CORS.
	CORSOrigins []string
	// Enqueuer queues classification after saves. Optional.
	Enqueuer Enqueuer
}

func New(o Options) *Server {
	s := &Server{q: o.Queries, log: o.Logger, ping: o.Ping, cors: map[string]bool{}, enqueue: o.Enqueuer}
	for _, origin := range o.CORSOrigins {
		s.cors[origin] = true
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	s.authMW = o.Verifier.Middleware(func(w http.ResponseWriter, r *http.Request, err error) {
		s.log.DebugContext(r.Context(), "auth rejected", "err", err)
		writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in again.")
	})
	return s
}

// Handler returns the router with logging and panic recovery applied.
func (s *Server) Handler() http.Handler {
	return s.routes(s.authMW)
}

func (s *Server) routes(authMW func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)

	v1 := http.NewServeMux()
	v1.HandleFunc("POST /v1/items", s.createItem)
	v1.HandleFunc("GET /v1/items", s.listItems)
	v1.HandleFunc("GET /v1/items/{id}", s.getItem)
	v1.HandleFunc("PATCH /v1/items/{id}", s.updateItem)
	v1.HandleFunc("DELETE /v1/items/{id}", s.deleteItem)
	v1.HandleFunc("GET /v1/categories", s.listCategories)
	v1.HandleFunc("POST /v1/categories", s.createCategory)
	v1.HandleFunc("PUT /v1/devices/{token}", s.registerDevice)
	v1.HandleFunc("DELETE /v1/devices/{token}", s.unregisterDevice)
	mux.Handle("/v1/", authMW(v1))

	return s.recoverer(s.logRequests(s.withCORS(mux)))
}

// withCORS answers preflight requests and adds CORS headers for allowed
// origins. Requests from other origins get no CORS headers, so browsers
// block them.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !s.cors[origin] {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Add("Vary", "Origin")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if s.ping != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.ping(ctx); err != nil {
			s.log.ErrorContext(r.Context(), "health check failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "unavailable", "Database unreachable.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.ErrorContext(r.Context(), "panic", "value", v)
				writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads a JSON body into dst, rejecting unknown fields and oversized
// bodies. It writes the error response itself and returns false on failure.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "Request body is too large.")
		} else {
			writeError(w, http.StatusBadRequest, "bad_json", "Request body must be valid JSON.")
		}
		return false
	}
	return true
}
