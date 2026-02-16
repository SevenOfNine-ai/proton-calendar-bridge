package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/auth"
	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type protonCalendarClient interface {
	GetCalendars(ctx context.Context) ([]protonapi.Calendar, error)
	GetCalendarEvents(ctx context.Context, id string, page, pageSize int) ([]protonapi.CalendarEvent, error)
}

type ProtonProvider struct {
	client protonCalendarClient
	store  auth.Store
}

func NewProtonProvider(client *protonapi.Client, store auth.Store) *ProtonProvider {
	return &ProtonProvider{client: client, store: store}
}

func (p *ProtonProvider) Name() string { return "proton" }

func (p *ProtonProvider) Capabilities(context.Context) (CapabilitySet, error) {
	return CapabilitySet{
		ReadOnly:        true,
		WriteSupported:  false,
		SharedCalendars: true,
		Attendees:       false,
		Reminders:       false,
		Recurrence:      false,
		Notes:           []string{"Proton provider is read-only during Phase 1."},
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
		out = append(out, domain.Calendar{
			ID:       c.ID,
			Name:     c.Name,
			ReadOnly: true,
			Shared:   c.Type != protonapi.CalendarTypeNormal,
			Permissions: []string{
				"read",
			},
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
	out := make([]domain.Event, 0, len(items))
	for _, item := range items {
		e := domain.Event{
			ID:         item.ID,
			CalendarID: item.CalendarID,
			Title:      "[encrypted]",
			Start:      time.Unix(item.StartTime, 0).UTC(),
			End:        time.Unix(item.EndTime, 0).UTC(),
			AllDay:     bool(item.FullDay),
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

func (p *ProtonProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "create_event"}
}

func (p *ProtonProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "update_event"}
}

func (p *ProtonProvider) DeleteEvent(context.Context, string) error {
	return NotSupportedError{Operation: "delete_event"}
}
