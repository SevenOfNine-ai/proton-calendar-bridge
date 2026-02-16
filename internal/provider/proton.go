package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
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
}

type ProtonProvider struct {
	client       protonCalendarClient
	store        auth.Store
	keyPassword  []byte
	keyrings     *auth.KeyringManager
	decryptor    *bridgecrypto.EventDecryptor
	mu           sync.RWMutex
	addressKR    *gopenpgp.KeyRing
	calendarKRs  map[string]*gopenpgp.KeyRing
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
		ReadOnly:        true,
		WriteSupported:  false,
		SharedCalendars: true,
		Attendees:       true,
		Reminders:       true,
		Recurrence:      true,
		Notes:           []string{"Proton provider is read-only during Phase 2."},
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
	items, err := p.client.GetCalendarEvents(ctx, calendarID, 0, 100)
	if err != nil {
		return nil, err
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
			continue
		}
		parsed, err := bridgecrypto.ParseVCalendar(dec.SharedData, dec.PersonalData)
		if err != nil {
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

func (p *ProtonProvider) addressKeyRing(ctx context.Context) (*gopenpgp.KeyRing, error) {
	p.mu.RLock()
	if p.addressKR != nil {
		defer p.mu.RUnlock()
		return p.addressKR, nil
	}
	p.mu.RUnlock()

	if p.keyrings == nil {
		return nil, fmt.Errorf("keyring manager is not configured")
	}
	kr, err := p.keyrings.UnlockAddressKeys(ctx, p.keyPassword)
	if err != nil {
		return nil, fmt.Errorf("unlock address keys: %w", err)
	}
	p.mu.Lock()
	p.addressKR = kr
	p.mu.Unlock()
	return kr, nil
}

func (p *ProtonProvider) calendarKeyRing(ctx context.Context, calendarID string) (*gopenpgp.KeyRing, error) {
	p.mu.RLock()
	if kr, ok := p.calendarKRs[calendarID]; ok && kr != nil {
		defer p.mu.RUnlock()
		return kr, nil
	}
	p.mu.RUnlock()

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
	p.calendarKRs[calendarID] = calKR
	p.mu.Unlock()
	return calKR, nil
}

func (p *ProtonProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "create_event"}
}

func (p *ProtonProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "update_event"}
}

func (p *ProtonProvider) DeleteEvent(context.Context, string) error {
	return NotSupportedError{Operation: "delete_event"}
}
