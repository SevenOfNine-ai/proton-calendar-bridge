package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
)

type fakeClient struct {
	resp *http.Response
	err  error
}

func (f fakeClient) Do(*http.Request) (*http.Response, error) { return f.resp, f.err }

func TestICSProviderIdentity(t *testing.T) {
	p := NewICSProvider("https://x", fakeClient{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("BEGIN:VCALENDAR\nEND:VCALENDAR"))}})
	if p.Name() != "ics" {
		t.Fatal("unexpected provider name")
	}
	cals, err := p.ListCalendars(context.Background())
	if err != nil || len(cals) != 1 || !cals[0].ReadOnly {
		t.Fatalf("unexpected calendars: %+v err=%v", cals, err)
	}
}

func TestICSProviderListEvents(t *testing.T) {
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:1\nSUMMARY:Meet\nDTSTART:20260212T100000Z\nDTEND:20260212T110000Z\nEND:VEVENT\nEND:VCALENDAR"
	p := NewICSProvider("https://x", fakeClient{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(ics))}})
	events, err := p.ListEvents(context.Background(), "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "1" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestICSProviderFilteringAndNotSupported(t *testing.T) {
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:1\nSUMMARY:A\nDTSTART:20260212\nDTEND:20260213\nEND:VEVENT\nEND:VCALENDAR"
	p := NewICSProvider("https://x", fakeClient{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(ics))}})
	from := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	events, err := p.ListEvents(context.Background(), "cal", from, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected filtered out")
	}
	if _, err := p.CreateEvent(context.Background(), domain.EventMutation{}); err == nil {
		t.Fatal("expected not supported")
	}
	if _, err := p.UpdateEvent(context.Background(), "1", domain.EventMutation{}); err == nil {
		t.Fatal("expected not supported")
	}
	if err := p.DeleteEvent(context.Background(), "1"); err == nil {
		t.Fatal("expected not supported")
	}
}

func TestICSProviderFetchErrors(t *testing.T) {
	p := NewICSProvider("://bad", fakeClient{})
	if _, err := p.ListEvents(context.Background(), "", time.Time{}, time.Time{}); err == nil {
		t.Fatal("expected URL build error")
	}

	p = NewICSProvider("https://x", fakeClient{resp: &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("x"))}})
	if _, err := p.ListEvents(context.Background(), "", time.Time{}, time.Time{}); err == nil {
		t.Fatal("expected status error")
	}
}

func TestParseICSTime(t *testing.T) {
	if _, _, err := parseICSTime("invalid"); err == nil {
		t.Fatal("expected invalid")
	}
	if _, allDay, err := parseICSTime("20260212"); err != nil || !allDay {
		t.Fatalf("expected all-day parse, got err=%v allDay=%v", err, allDay)
	}
}

func TestICSProviderCapabilities(t *testing.T) {
	p := NewICSProvider("https://x", fakeClient{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("BEGIN:VCALENDAR\nEND:VCALENDAR"))}})
	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.ReadOnly || caps.WriteSupported {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}
