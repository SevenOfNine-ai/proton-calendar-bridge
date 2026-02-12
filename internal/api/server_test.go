package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/provider"
	"github.com/sevenofnine/proton-calendar-bridge/internal/security"
)

type fakeProvider struct{}

func (fakeProvider) Name() string { return "fake" }
func (fakeProvider) ListCalendars(context.Context) ([]domain.Calendar, error) {
	return []domain.Calendar{{ID: "1", Name: "x"}}, nil
}
func (fakeProvider) ListEvents(context.Context, string, time.Time, time.Time) ([]domain.Event, error) {
	return []domain.Event{{ID: "e1", Title: "E"}}, nil
}
func (fakeProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, provider.NotSupportedError{Operation: "create"}
}
func (fakeProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{ID: "u1"}, nil
}
func (fakeProvider) DeleteEvent(context.Context, string) error { return errors.New("delete failed") }

func TestServerRoutesAndAuth(t *testing.T) {
	s := New(Options{Provider: fakeProvider{}, Auth: security.BearerAuth{Enabled: true, Token: "t"}})
	ts := httptest.NewServer(s.httpSrv.Handler)
	defer ts.Close()

	res, _ := http.Get(ts.URL + "/healthz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", res.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/calendars", nil)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", res.StatusCode)
	}

	req.Header.Set("Authorization", "Bearer t")
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}
}

func TestServerMutationsAndMethods(t *testing.T) {
	s := New(Options{Provider: fakeProvider{}, Auth: security.BearerAuth{Enabled: false}})
	ts := httptest.NewServer(s.httpSrv.Handler)
	defer ts.Close()

	res, _ := http.Post(ts.URL+"/v1/events/create", "application/json", bytes.NewBufferString(`{"mutation":{}}`))
	if res.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 got %d", res.StatusCode)
	}

	res, _ = http.Post(ts.URL+"/v1/events/update", "application/json", bytes.NewBufferString(`{"event_id":"1","mutation":{}}`))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	res, _ = http.Post(ts.URL+"/v1/events/delete", "application/json", bytes.NewBufferString(`{"event_id":"1"}`))
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", res.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/v1/events", nil)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 got %d", res.StatusCode)
	}
}

func TestHelpersAndServeValidation(t *testing.T) {
	r := httptest.NewRecorder()
	writeErr(r, 400, "x")
	if r.Code != 400 {
		t.Fatal("wrong status")
	}
	var m map[string]string
	_ = json.Unmarshal(r.Body.Bytes(), &m)
	if m["error"] != "x" {
		t.Fatal("wrong payload")
	}

	s := New(Options{Provider: fakeProvider{}})
	if err := s.ServeTCP(context.Background(), ""); err == nil {
		t.Fatal("expected bind error")
	}
	if err := s.ServeUnix(context.Background(), ""); err == nil {
		t.Fatal("expected unix path error")
	}

	r = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/events/create", io.NopCloser(bytes.NewBufferString("{")))
	s.handleCreateEvent(r, req)
	if r.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", r.Code)
	}
}

type errProvider struct{}

func (errProvider) Name() string { return "err" }
func (errProvider) ListCalendars(context.Context) ([]domain.Calendar, error) {
	return nil, errors.New("boom")
}
func (errProvider) ListEvents(context.Context, string, time.Time, time.Time) ([]domain.Event, error) {
	return nil, errors.New("boom")
}
func (errProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, errors.New("boom")
}
func (errProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, errors.New("boom")
}
func (errProvider) DeleteEvent(context.Context, string) error { return nil }

func TestServerErrorPaths(t *testing.T) {
	s := New(Options{Provider: errProvider{}, Auth: security.BearerAuth{Enabled: false}})
	ts := httptest.NewServer(s.httpSrv.Handler)
	defer ts.Close()

	res, _ := http.Get(ts.URL + "/v1/calendars")
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", res.StatusCode)
	}
	res, _ = http.Get(ts.URL + "/v1/events?from=bad")
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", res.StatusCode)
	}

	res, _ = http.Post(ts.URL+"/v1/events/update", "application/json", bytes.NewBufferString(`{"event_id":"1","mutation":{}}`))
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", res.StatusCode)
	}
}

func TestServeTCPAndUnixLifecycle(t *testing.T) {
	s := New(Options{Provider: fakeProvider{}, Auth: security.BearerAuth{Enabled: false}})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	if err := s.ServeTCP(ctx, "127.0.0.1:0"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("ServeTCP err=%v", err)
	}

	s = New(Options{Provider: fakeProvider{}, Auth: security.BearerAuth{Enabled: false}})
	ctx, cancel = context.WithCancel(context.Background())
	sock := t.TempDir() + "/bridge.sock"
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	if err := s.ServeUnix(ctx, sock); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("ServeUnix err=%v", err)
	}
}
