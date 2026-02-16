package provider

import (
	"context"
	"testing"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/sevenofnine/proton-calendar-bridge/internal/auth"
	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type fakeProtonClient struct {
	calendars []protonapi.Calendar
	events    []protonapi.CalendarEvent
	err       error
}

func (f fakeProtonClient) GetCalendars(context.Context) ([]protonapi.Calendar, error) {
	return f.calendars, f.err
}

func (f fakeProtonClient) GetCalendarEvents(context.Context, string, int, int) ([]protonapi.CalendarEvent, error) {
	return f.events, f.err
}

func TestProtonProviderMapping(t *testing.T) {
	t.Parallel()

	p := &ProtonProvider{
		client: fakeProtonClient{
			calendars: []protonapi.Calendar{{ID: "cal-1", Name: "Work", Type: proton.CalendarType(1)}},
			events:    []protonapi.CalendarEvent{{ID: "e1", CalendarID: "cal-1", StartTime: 1700000000, EndTime: 1700003600, FullDay: proton.Bool(false)}},
		},
		store: auth.Store{},
	}

	if p.Name() != "proton" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
	calendars, err := p.ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("list calendars: %v", err)
	}
	if len(calendars) != 1 || !calendars[0].Shared || !calendars[0].ReadOnly {
		t.Fatalf("unexpected calendars: %+v", calendars)
	}

	from := time.Unix(1699999000, 0).UTC()
	to := time.Unix(1700007200, 0).UTC()
	events, err := p.ListEvents(context.Background(), "cal-1", from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].Title != "[encrypted]" {
		t.Fatalf("unexpected events: %+v", events)
	}

	if _, err := p.CreateEvent(context.Background(), domain.EventMutation{}); err == nil {
		t.Fatal("expected not supported")
	}
	if _, err := p.UpdateEvent(context.Background(), "e1", domain.EventMutation{}); err == nil {
		t.Fatal("expected not supported")
	}
	if err := p.DeleteEvent(context.Background(), "e1"); err == nil {
		t.Fatal("expected not supported")
	}
}
