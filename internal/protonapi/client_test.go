package protonapi

import (
	"context"
	"net/url"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
)

type fakeManager struct {
	newClient           *proton.Client
	loginClient         *proton.Client
	loginAuth           proton.Auth
	loginErr            error
	refreshClient       *proton.Client
	refreshAuth         proton.Auth
	refreshErr          error
	newClientCalledWith struct{ uid, acc, ref string }
}

func (f *fakeManager) NewClient(uid, acc, ref string) *proton.Client {
	f.newClientCalledWith.uid = uid
	f.newClientCalledWith.acc = acc
	f.newClientCalledWith.ref = ref
	return f.newClient
}

func (f *fakeManager) NewClientWithLogin(context.Context, string, []byte) (*proton.Client, proton.Auth, error) {
	return f.loginClient, f.loginAuth, f.loginErr
}

func (f *fakeManager) NewClientWithRefresh(context.Context, string, string) (*proton.Client, proton.Auth, error) {
	return f.refreshClient, f.refreshAuth, f.refreshErr
}

type fakeCalendarClient struct {
	auth2FAErr error
	calendars  []proton.Calendar
	calendar   proton.Calendar
	keys       proton.CalendarKeys
	members    []proton.CalendarMember
	passphrase proton.CalendarPassphrase
	events     []proton.CalendarEvent
	event      proton.CalendarEvent
	err        error
}

func (f *fakeCalendarClient) Auth2FA(context.Context, proton.Auth2FAReq) error { return f.auth2FAErr }
func (f *fakeCalendarClient) GetCalendars(context.Context) ([]proton.Calendar, error) {
	return f.calendars, f.err
}
func (f *fakeCalendarClient) GetCalendar(context.Context, string) (proton.Calendar, error) {
	return f.calendar, f.err
}
func (f *fakeCalendarClient) GetCalendarKeys(context.Context, string) (proton.CalendarKeys, error) {
	return f.keys, f.err
}
func (f *fakeCalendarClient) GetCalendarMembers(context.Context, string) ([]proton.CalendarMember, error) {
	return f.members, f.err
}
func (f *fakeCalendarClient) GetCalendarPassphrase(context.Context, string) (proton.CalendarPassphrase, error) {
	return f.passphrase, f.err
}
func (f *fakeCalendarClient) GetCalendarEvents(context.Context, string, int, int, url.Values) ([]proton.CalendarEvent, error) {
	return f.events, f.err
}
func (f *fakeCalendarClient) GetCalendarEvent(context.Context, string, string) (proton.CalendarEvent, error) {
	return f.event, f.err
}

func TestClientSetSessionUsesManager(t *testing.T) {
	mgr := &fakeManager{}
	client := NewClient(ClientOptions{Manager: mgr})
	client.SetSession(Auth{UID: "uid", AccessToken: "acc", RefreshToken: "ref"})
	if mgr.newClientCalledWith.uid != "uid" || mgr.newClientCalledWith.acc != "acc" || mgr.newClientCalledWith.ref != "ref" {
		t.Fatalf("unexpected new client call: %+v", mgr.newClientCalledWith)
	}
}

func TestClientCalendarMethodsAndStatus(t *testing.T) {
	fc := &fakeCalendarClient{
		calendars: []proton.Calendar{{ID: "c1"}},
		calendar:  proton.Calendar{ID: "c1"},
		keys:      proton.CalendarKeys{{ID: "k1"}},
		members:   []proton.CalendarMember{{ID: "m1"}},
		passphrase: proton.CalendarPassphrase{
			ID: "p1",
		},
		events: []proton.CalendarEvent{{ID: "e1", CalendarID: "c1"}},
		event:  proton.CalendarEvent{ID: "e1", CalendarID: "c1"},
	}
	c := &Client{client: fc, status: StatusUnknown}

	if _, err := c.GetCalendars(context.Background()); err != nil {
		t.Fatalf("GetCalendars: %v", err)
	}
	if _, err := c.GetCalendar(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCalendar: %v", err)
	}
	if _, err := c.GetCalendarKeys(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCalendarKeys: %v", err)
	}
	if _, err := c.GetCalendarMembers(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCalendarMembers: %v", err)
	}
	if _, err := c.GetCalendarPassphrase(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCalendarPassphrase: %v", err)
	}
	if _, err := c.GetCalendarEvents(context.Background(), "c1", 0, 10); err != nil {
		t.Fatalf("GetCalendarEvents: %v", err)
	}
	if _, err := c.GetCalendarEvent(context.Background(), "c1", "e1"); err != nil {
		t.Fatalf("GetCalendarEvent: %v", err)
	}
	if c.Status() != StatusConnected {
		t.Fatalf("expected connected status, got %s", c.Status())
	}
}

func TestClientRequiresSession(t *testing.T) {
	c := &Client{}
	if _, err := c.GetCalendars(context.Background()); err == nil {
		t.Fatal("expected missing session error")
	}
}
