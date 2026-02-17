package provider

import (
	"context"
	"testing"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/sevenofnine/proton-calendar-bridge/internal/auth"
	bridgecrypto "github.com/sevenofnine/proton-calendar-bridge/internal/crypto"
	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type fakeProtonClient struct {
	calendars            []protonapi.Calendar
	events               []protonapi.CalendarEvent
	eventPages           map[int][]protonapi.CalendarEvent
	members              []protonapi.CalendarMember
	passphrase           protonapi.CalendarPassphrase
	keys                 protonapi.CalendarKeys
	addresses            []protonapi.Address
	err                  error
	calendarMembersCalls int
	passphraseCalls      int
	keysCalls            int
	eventsCalls          int
}

func (f *fakeProtonClient) GetCalendars(context.Context) ([]protonapi.Calendar, error) {
	return f.calendars, f.err
}
func (f *fakeProtonClient) GetCalendarEvents(_ context.Context, _ string, page, _ int) ([]protonapi.CalendarEvent, error) {
	f.eventsCalls++
	if f.eventPages != nil {
		return f.eventPages[page], f.err
	}
	return f.events, f.err
}
func (f *fakeProtonClient) GetCalendarMembers(context.Context, string) ([]protonapi.CalendarMember, error) {
	f.calendarMembersCalls++
	return f.members, f.err
}
func (f *fakeProtonClient) GetCalendarPassphrase(context.Context, string) (protonapi.CalendarPassphrase, error) {
	f.passphraseCalls++
	return f.passphrase, f.err
}
func (f *fakeProtonClient) GetCalendarKeys(context.Context, string) (protonapi.CalendarKeys, error) {
	f.keysCalls++
	return f.keys, f.err
}
func (f *fakeProtonClient) GetAddresses(context.Context) ([]protonapi.Address, error) {
	return f.addresses, f.err
}

func TestProtonProviderMappingAndEvents(t *testing.T) {
	t.Parallel()

	fake := &fakeProtonClient{
		calendars: []protonapi.Calendar{{ID: "cal-1", Name: "Work", Type: proton.CalendarType(1)}},
		members:   []protonapi.CalendarMember{{ID: "m1", Permissions: proton.CalendarPermissions(1)}},
		events: []protonapi.CalendarEvent{{
			ID:         "e1",
			CalendarID: "cal-1",
			SharedEvents: []proton.CalendarEventPart{{
				Type: proton.CalendarEventTypeClear,
				Data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Decrypted\nDTSTART:20260216T090000Z\nDTEND:20260216T100000Z\nATTENDEE:mailto:test@example.com\nEND:VEVENT\nEND:VCALENDAR",
			}},
			PersonalEvents: []proton.CalendarEventPart{{
				Type: proton.CalendarEventTypeClear,
				Data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nBEGIN:VALARM\nTRIGGER:-PT5M\nEND:VALARM\nEND:VEVENT\nEND:VCALENDAR",
			}},
		}},
	}

	kr, err := gopenpgp.NewKeyRing(nil)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}

	p := &ProtonProvider{
		client:      fake,
		store:       auth.Store{},
		decryptor:   &bridgecrypto.EventDecryptor{},
		calendarKRs: map[string]*gopenpgp.KeyRing{"cal-1": kr},
		addressKR:   kr,
	}

	if p.Name() != "proton" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
	calendars, err := p.ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("list calendars: %v", err)
	}
	if len(calendars) != 1 || !calendars[0].Shared || calendars[0].ReadOnly {
		t.Fatalf("unexpected calendars: %+v", calendars)
	}

	from := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 16, 11, 0, 0, 0, time.UTC)
	events, err := p.ListEvents(context.Background(), "cal-1", from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].Title != "Decrypted" || len(events[0].Reminders) != 1 {
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

func TestProtonProviderCalendarKeyringCache(t *testing.T) {
	t.Parallel()

	kr, err := gopenpgp.NewKeyRing(nil)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	fake := &fakeProtonClient{}
	p := &ProtonProvider{client: fake, calendarKRs: map[string]*gopenpgp.KeyRing{"cal-1": kr}}

	first, err := p.calendarKeyRing(context.Background(), "cal-1")
	if err != nil {
		t.Fatalf("first keyring lookup: %v", err)
	}
	second, err := p.calendarKeyRing(context.Background(), "cal-1")
	if err != nil {
		t.Fatalf("second keyring lookup: %v", err)
	}
	if first != second {
		t.Fatal("expected cached keyring instance")
	}
	if fake.passphraseCalls != 0 || fake.keysCalls != 0 {
		t.Fatalf("expected no key derivation calls, got passphrase=%d keys=%d", fake.passphraseCalls, fake.keysCalls)
	}
}

func TestProtonProviderListEventsPaginationAndParseDegrade(t *testing.T) {
	t.Parallel()

	kr, err := gopenpgp.NewKeyRing(nil)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}

	validEvent := protonapi.CalendarEvent{
		ID:         "e-valid",
		CalendarID: "cal-1",
		SharedEvents: []proton.CalendarEventPart{{
			Type: proton.CalendarEventTypeClear,
			Data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Paged\nDTSTART:20260216T090000Z\nDTEND:20260216T100000Z\nEND:VEVENT\nEND:VCALENDAR",
		}},
	}
	invalidEvent := protonapi.CalendarEvent{
		ID:         "e-invalid",
		CalendarID: "cal-1",
		StartTime:  1771232400,
		EndTime:    1771236000,
		SharedEvents: []proton.CalendarEventPart{{
			Type: proton.CalendarEventTypeClear,
			Data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Broken\nEND:VEVENT\nEND:VCALENDAR",
		}},
	}

	page0 := make([]protonapi.CalendarEvent, 0, 100)
	for i := 0; i < 99; i++ {
		page0 = append(page0, validEvent)
	}
	page0 = append(page0, invalidEvent)

	fake := &fakeProtonClient{
		eventPages: map[int][]protonapi.CalendarEvent{
			0: page0,
			1: []protonapi.CalendarEvent{validEvent},
			2: []protonapi.CalendarEvent{},
		},
	}

	p := &ProtonProvider{
		client:      fake,
		store:       auth.Store{},
		decryptor:   &bridgecrypto.EventDecryptor{},
		calendarKRs: map[string]*gopenpgp.KeyRing{"cal-1": kr},
		addressKR:   kr,
	}

	events, err := p.ListEvents(context.Background(), "cal-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if fake.eventsCalls != 2 {
		t.Fatalf("expected 2 paged event calls, got %d", fake.eventsCalls)
	}
	if len(events) != 101 {
		t.Fatalf("expected 101 events, got %d", len(events))
	}

	var degraded bool
	for _, e := range events {
		if e.ID == "e-invalid" {
			degraded = true
			if e.Title != "[parse error]" {
				t.Fatalf("expected parse error degraded title, got %q", e.Title)
			}
		}
	}
	if !degraded {
		t.Fatal("expected degraded event in output")
	}
}
