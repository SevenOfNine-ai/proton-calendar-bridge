package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/provider"
	"github.com/sevenofnine/proton-calendar-bridge/internal/security"
)

type Server struct {
	provider provider.CalendarProvider
	auth     security.BearerAuth
	log      *slog.Logger
	httpSrv  *http.Server
}

type Options struct {
	Provider provider.CalendarProvider
	Auth     security.BearerAuth
	Logger   *slog.Logger
}

func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{provider: opts.Provider, auth: opts.Auth, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/calendars", s.handleCalendars)
	mux.HandleFunc("/v1/events", s.handleEvents)
	mux.HandleFunc("/v1/events/create", s.handleCreateEvent)
	mux.HandleFunc("/v1/events/update", s.handleUpdateEvent)
	mux.HandleFunc("/v1/events/delete", s.handleDeleteEvent)
	s.httpSrv = &http.Server{Handler: s.wrapAuth(mux), ReadHeaderTimeout: 5 * time.Second}
	return s
}

func (s *Server) ServeTCP(ctx context.Context, bind string) error {
	if bind == "" {
		return errors.New("bind required")
	}
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}
	go s.shutdownOnContext(ctx)
	return s.httpSrv.Serve(ln)
}

func (s *Server) ServeUnix(ctx context.Context, path string) error {
	if path == "" {
		return errors.New("socket path required")
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	go s.shutdownOnContext(ctx)
	return s.httpSrv.Serve(ln)
}

func (s *Server) wrapAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && !s.auth.Authorize(r) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) shutdownOnContext(ctx context.Context) {
	<-ctx.Done()
	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.httpSrv.Shutdown(timeout)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "provider": s.provider.Name()})
}

func (s *Server) handleCalendars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, err := s.provider.ListCalendars(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	calendarID := r.URL.Query().Get("calendar_id")
	from, _ := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	items, err := s.provider.ListEvents(r.Context(), calendarID, from, to)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	s.handleMutation(w, r, func(ctx context.Context, payload mutationRequest) (any, error) {
		return s.provider.CreateEvent(ctx, payload.Mutation)
	})
}

func (s *Server) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	s.handleMutation(w, r, func(ctx context.Context, payload mutationRequest) (any, error) {
		return s.provider.UpdateEvent(ctx, payload.EventID, payload.Mutation)
	})
}

func (s *Server) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	s.handleMutation(w, r, func(ctx context.Context, payload mutationRequest) (any, error) {
		return map[string]string{"event_id": payload.EventID}, s.provider.DeleteEvent(ctx, payload.EventID)
	})
}

type mutationRequest struct {
	EventID  string               `json:"event_id"`
	Mutation domain.EventMutation `json:"mutation"`
}

func (s *Server) handleMutation(w http.ResponseWriter, r *http.Request, run func(context.Context, mutationRequest) (any, error)) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload mutationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	out, err := run(r.Context(), payload)
	if err != nil {
		if errors.Is(err, provider.ErrNotSupported) {
			writeErr(w, http.StatusNotImplemented, err.Error())
			return
		}
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
