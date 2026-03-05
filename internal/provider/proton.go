package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/google/uuid"
	"github.com/sevenofnine/proton-calendar-bridge/internal/auth"
	bridgecrypto "github.com/sevenofnine/proton-calendar-bridge/internal/crypto"
	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type protonCalendarClient interface {
	GetCalendars(ctx context.Context) ([]protonapi.Calendar, error)
	GetCalendarMembers(ctx context.Context, id string) ([]protonapi.CalendarMember, error)
	GetCalendarEvents(ctx context.Context, id string, page, pageSize int) ([]protonapi.CalendarEvent, error)
	GetCalendarPassphrase(ctx context.Context, id string) (protonapi.CalendarPassphrase, error)
	GetCalendarKeys(ctx context.Context, id string) (protonapi.CalendarKeys, error)
	GetAddresses(ctx context.Context) ([]protonapi.Address, error)
	CreateCalendarEvent(ctx context.Context, calendarID string, req protonapi.CreateCalendarEventReq) (protonapi.CalendarEvent, error)
	UpdateCalendarEvent(ctx context.Context, calendarID, eventID string, req protonapi.CreateCalendarEventReq) (protonapi.CalendarEvent, error)
	DeleteCalendarEvent(ctx context.Context, calendarID, eventID string) error
}

type ProtonProvider struct {
	client      protonCalendarClient
	store       auth.Store
	keyPassword []byte
	keyrings    *auth.KeyringManager
	decryptor   *bridgecrypto.EventDecryptor
	mu          sync.RWMutex
	addressKR   *gopenpgp.KeyRing
	calendarKRs map[string]*gopenpgp.KeyRing
}

func NewProtonProvider(client *protonapi.Client, store auth.Store) *ProtonProvider {
	return NewProtonProviderWithKeyPassword(client, store, nil)
}

func NewProtonProviderWithKeyPassword(client protonCalendarClient, store auth.Store, keyPassword []byte) *ProtonProvider {
	return &ProtonProvider{
		client:      client,
		store:       store,
		keyPassword: keyPassword,
		keyrings:    auth.NewKeyringManager(client),
		decryptor:   &bridgecrypto.EventDecryptor{},
		calendarKRs: make(map[string]*gopenpgp.KeyRing),
	}
}

func (p *ProtonProvider) Name() string { return "proton" }

func (p *ProtonProvider) Capabilities(context.Context) (CapabilitySet, error) {
	return CapabilitySet{
		ReadOnly:        false,
		WriteSupported:  true,
		SharedCalendars: true,
		Attendees:       true,
		Reminders:       true,
		Recurrence:      true,
	}, nil
}

func (p *ProtonProvider) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	if p.client == nil {
		return nil, fmt.Errorf("proton client is not configured")
	}
	items, err := p.client.GetCalendars(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Calendar, 0, len(items))
	for _, c := range items {
		members, err := p.client.GetCalendarMembers(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("get calendar members for %s: %w", c.ID, err)
		}
		permissions := []string{"read"}
		readOnly := true
		if len(members) > 0 && int(members[0].Permissions) > 0 {
			permissions = append(permissions, "write")
			readOnly = false
		}
		out = append(out, domain.Calendar{
			ID:          c.ID,
			Name:        c.Name,
			ReadOnly:    readOnly,
			Shared:      c.Type != protonapi.CalendarTypeNormal,
			Permissions: permissions,
		})
	}
	return out, nil
}

func (p *ProtonProvider) ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]domain.Event, error) {
	if p.client == nil {
		return nil, fmt.Errorf("proton client is not configured")
	}
	if calendarID == "" {
		return nil, fmt.Errorf("calendar id is required")
	}
	const pageSize = 100
	var items []protonapi.CalendarEvent
	for page := 0; ; page++ {
		batch, err := p.client.GetCalendarEvents(ctx, calendarID, page, pageSize)
		if err != nil {
			return nil, err
		}
		items = append(items, batch...)
		if len(batch) < pageSize {
			break
		}
	}

	calKR, err := p.calendarKeyRing(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	addrKR, err := p.addressKeyRing(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Event, 0, len(items))
	for _, item := range items {
		dec, err := p.decryptor.DecryptEvent(item, calKR, addrKR)
		if err != nil {
			slog.Warn("failed to decrypt event", "event_id", item.ID, "error", err)
			out = append(out, domain.Event{
				ID:         item.ID,
				CalendarID: item.CalendarID,
				Title:      "[decrypt error]",
				Start:      time.Unix(item.StartTime, 0).UTC(),
				End:        time.Unix(item.EndTime, 0).UTC(),
				AllDay:     bool(item.FullDay),
			})
			continue
		}
		parsed, err := bridgecrypto.ParseVCalendar(dec.SharedData, dec.PersonalData)
		if err != nil {
			slog.Warn("failed to parse event", "event_id", item.ID, "error", err)
			out = append(out, domain.Event{
				ID:         item.ID,
				CalendarID: item.CalendarID,
				Title:      "[parse error]",
				Start:      time.Unix(item.StartTime, 0).UTC(),
				End:        time.Unix(item.EndTime, 0).UTC(),
				AllDay:     bool(item.FullDay),
			})
			continue
		}

		e := domain.Event{
			ID:          item.ID,
			CalendarID:  item.CalendarID,
			Title:       parsed.Title,
			Description: parsed.Description,
			Location:    parsed.Location,
			Start:       parsed.Start,
			End:         parsed.End,
			AllDay:      parsed.AllDay,
			Recurrence:  parsed.Recurrence,
			Attendees:   parsed.Attendees,
			Reminders:   parsed.Reminders,
		}
		if !from.IsZero() && e.End.Before(from) {
			continue
		}
		if !to.IsZero() && e.Start.After(to) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// CreateEvent encrypts the event mutation and submits it to the Proton Calendar API.
func (p *ProtonProvider) CreateEvent(ctx context.Context, in domain.EventMutation) (domain.Event, error) {
	if p.client == nil {
		return domain.Event{}, fmt.Errorf("proton client is not configured")
	}
	if in.CalendarID == "" {
		return domain.Event{}, fmt.Errorf("calendar_id is required")
	}

	calKR, err := p.calendarKeyRing(ctx, in.CalendarID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get calendar key ring: %w", err)
	}
	addrKR, err := p.addressKeyRing(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get address key ring: %w", err)
	}
	memberID, err := p.calendarMemberID(ctx, in.CalendarID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get member id: %w", err)
	}

	uid := uuid.New().String() + "@proton.me"
	req, err := p.buildEventReq(in, uid, calKR, addrKR, memberID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("build event request: %w", err)
	}

	created, err := p.client.CreateCalendarEvent(ctx, in.CalendarID, req)
	if err != nil {
		return domain.Event{}, fmt.Errorf("create event: %w", err)
	}

	return protonEventToDomain(created, in), nil
}

// UpdateEvent re-encrypts the updated event and submits it to the Proton Calendar API.
func (p *ProtonProvider) UpdateEvent(ctx context.Context, eventID string, in domain.EventMutation) (domain.Event, error) {
	if p.client == nil {
		return domain.Event{}, fmt.Errorf("proton client is not configured")
	}
	if eventID == "" {
		return domain.Event{}, fmt.Errorf("event_id is required")
	}
	if in.CalendarID == "" {
		return domain.Event{}, fmt.Errorf("calendar_id is required")
	}

	calKR, err := p.calendarKeyRing(ctx, in.CalendarID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get calendar key ring: %w", err)
	}
	addrKR, err := p.addressKeyRing(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get address key ring: %w", err)
	}
	memberID, err := p.calendarMemberID(ctx, in.CalendarID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get member id: %w", err)
	}

	// Parse calendarID from composite eventID if present.
	calendarID := in.CalendarID
	rawEventID := eventID
	if cID, eID, err := splitCalendarEventID(eventID); err == nil {
		calendarID = cID
		rawEventID = eID
	}

	uid := uuid.New().String() + "@proton.me"
	req, err := p.buildEventReq(in, uid, calKR, addrKR, memberID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("build event request: %w", err)
	}

	updated, err := p.client.UpdateCalendarEvent(ctx, calendarID, rawEventID, req)
	if err != nil {
		return domain.Event{}, fmt.Errorf("update event: %w", err)
	}

	return protonEventToDomain(updated, in), nil
}

// DeleteEvent removes an event from the Proton Calendar.
// eventID must be in "calendarID:eventID" format (as returned by CreateEvent/UpdateEvent).
func (p *ProtonProvider) DeleteEvent(ctx context.Context, eventID string) error {
	if p.client == nil {
		return fmt.Errorf("proton client is not configured")
	}
	if eventID == "" {
		return fmt.Errorf("event_id is required")
	}
	calendarID, rawEventID, err := splitCalendarEventID(eventID)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return p.client.DeleteCalendarEvent(ctx, calendarID, rawEventID)
}

// buildEventReq assembles a CreateCalendarEventReq from an EventMutation by
// encoding the VCALENDAR text and encrypting it for the Proton API.
func (p *ProtonProvider) buildEventReq(
	in domain.EventMutation,
	uid string,
	calKR, addrKR *gopenpgp.KeyRing,
	memberID string,
) (protonapi.CreateCalendarEventReq, error) {
	sharedVCal := bridgecrypto.EncodeSharedVCalendar(in, uid)
	personalVCal := bridgecrypto.EncodePersonalVCalendar(in.Reminders, uid)

	encShared, err := bridgecrypto.EncryptSharedEvent(sharedVCal, calKR, addrKR)
	if err != nil {
		return protonapi.CreateCalendarEventReq{}, fmt.Errorf("encrypt shared event: %w", err)
	}

	encPersonal, err := bridgecrypto.EncryptPersonalEvent(personalVCal, addrKR)
	if err != nil {
		return protonapi.CreateCalendarEventReq{}, fmt.Errorf("encrypt personal event: %w", err)
	}

	fullDay := 0
	if in.AllDay {
		fullDay = 1
	}

	req := protonapi.CreateCalendarEventReq{
		MajorVersion:    1,
		UID:             uid,
		IsOrganizer:     1,
		Permissions:     3,
		SharedKeyPacket: encShared.KeyPacket,
		SharedEventContent: []protonapi.CalendarEventPartReq{
			{
				Type:      3, // encrypted + signed
				Data:      encShared.DataPacket,
				Signature: encShared.Signature,
			},
		},
		StartTime:     in.Start.Unix(),
		StartTimezone: "UTC",
		EndTime:       in.End.Unix(),
		EndTimezone:   "UTC",
		FullDay:       fullDay,
		RRule:         in.Recurrence,
	}

	if encPersonal.Data != "" {
		req.PersonalEventContent = []protonapi.CalendarEventPartReq{
			{
				MemberID:  memberID,
				Type:      3, // encrypted + signed
				Data:      encPersonal.Data,
				Signature: encPersonal.Signature,
			},
		}
	}

	return req, nil
}

// calendarMemberID returns the first member ID for a calendar.
func (p *ProtonProvider) calendarMemberID(ctx context.Context, calendarID string) (string, error) {
	members, err := p.client.GetCalendarMembers(ctx, calendarID)
	if err != nil {
		return "", fmt.Errorf("get calendar members: %w", err)
	}
	if len(members) == 0 {
		return "", fmt.Errorf("calendar %s has no members", calendarID)
	}
	return members[0].ID, nil
}

// protonEventToDomain converts a CalendarEvent returned by the Proton API
// (after create/update) into a domain.Event. Fields not echoed back by the API
// are filled from the original mutation.
func protonEventToDomain(e protonapi.CalendarEvent, in domain.EventMutation) domain.Event {
	start := time.Unix(e.StartTime, 0).UTC()
	end := time.Unix(e.EndTime, 0).UTC()
	if start.IsZero() {
		start = in.Start
	}
	if end.IsZero() {
		end = in.End
	}
	now := time.Now().UTC()
	// Encode as "calendarID:eventID" so Delete can reconstruct the API path.
	id := e.ID
	if e.CalendarID != "" {
		id = e.CalendarID + ":" + e.ID
	}
	return domain.Event{
		ID:          id,
		CalendarID:  e.CalendarID,
		Title:       in.Title,
		Description: in.Description,
		Location:    in.Location,
		Start:       start,
		End:         end,
		AllDay:      bool(e.FullDay),
		Recurrence:  in.Recurrence,
		Attendees:   in.Attendees,
		Reminders:   in.Reminders,
		UpdatedAt:   &now,
	}
}

// splitCalendarEventID parses a "calendarID:eventID" compound identifier.
func splitCalendarEventID(id string) (calendarID, eventID string, err error) {
	for i, c := range id {
		if c == ':' {
			return id[:i], id[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("event id %q must be in 'calendarID:eventID' format", id)
}

func (p *ProtonProvider) addressKeyRing(ctx context.Context) (*gopenpgp.KeyRing, error) {
	p.mu.RLock()
	if p.addressKR != nil {
		defer p.mu.RUnlock()
		return p.addressKR, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if p.addressKR != nil {
		defer p.mu.Unlock()
		return p.addressKR, nil
	}
	p.mu.Unlock()

	if p.keyrings == nil {
		return nil, fmt.Errorf("keyring manager is not configured")
	}
	kr, err := p.keyrings.UnlockAddressKeys(ctx, p.keyPassword)
	if err != nil {
		return nil, fmt.Errorf("unlock address keys: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.addressKR != nil {
		return p.addressKR, nil
	}
	p.addressKR = kr
	return kr, nil
}

func (p *ProtonProvider) calendarKeyRing(ctx context.Context, calendarID string) (*gopenpgp.KeyRing, error) {
	p.mu.RLock()
	if kr, ok := p.calendarKRs[calendarID]; ok && kr != nil {
		defer p.mu.RUnlock()
		return kr, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if kr, ok := p.calendarKRs[calendarID]; ok && kr != nil {
		defer p.mu.Unlock()
		return kr, nil
	}
	p.mu.Unlock()

	addrKR, err := p.addressKeyRing(ctx)
	if err != nil {
		return nil, err
	}

	members, err := p.client.GetCalendarMembers(ctx, calendarID)
	if err != nil {
		return nil, fmt.Errorf("get calendar members: %w", err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("calendar %s has no members", calendarID)
	}
	memberID := members[0].ID

	passphrase, err := p.client.GetCalendarPassphrase(ctx, calendarID)
	if err != nil {
		return nil, fmt.Errorf("get calendar passphrase: %w", err)
	}
	calendarPassphrase, err := passphrase.Decrypt(memberID, addrKR)
	if err != nil {
		return nil, fmt.Errorf("decrypt calendar passphrase: %w", err)
	}

	keys, err := p.client.GetCalendarKeys(ctx, calendarID)
	if err != nil {
		return nil, fmt.Errorf("get calendar keys: %w", err)
	}
	calKR, err := keys.Unlock(calendarPassphrase)
	if err != nil {
		return nil, fmt.Errorf("unlock calendar keys: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if kr, ok := p.calendarKRs[calendarID]; ok && kr != nil {
		return kr, nil
	}
	p.calendarKRs[calendarID] = calKR
	return calKR, nil
}
