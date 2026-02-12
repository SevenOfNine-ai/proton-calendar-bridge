package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
)

var ErrNotSupported = errors.New("operation not supported by provider")

type CalendarProvider interface {
	Name() string
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]domain.Event, error)
	CreateEvent(ctx context.Context, in domain.EventMutation) (domain.Event, error)
	UpdateEvent(ctx context.Context, eventID string, in domain.EventMutation) (domain.Event, error)
	DeleteEvent(ctx context.Context, eventID string) error
}

type CapabilitySet struct {
	ReadOnly        bool     `json:"read_only"`
	WriteSupported  bool     `json:"write_supported"`
	SharedCalendars bool     `json:"shared_calendars"`
	Attendees       bool     `json:"attendees"`
	Reminders       bool     `json:"reminders"`
	Recurrence      bool     `json:"recurrence"`
	Notes           []string `json:"notes,omitempty"`
}

type CapabilityProvider interface {
	Capabilities(ctx context.Context) (CapabilitySet, error)
}

type NotSupportedError struct {
	Operation string
}

func (e NotSupportedError) Error() string {
	if e.Operation == "" {
		return ErrNotSupported.Error()
	}
	return fmt.Sprintf("%s: %v", e.Operation, ErrNotSupported)
}

func (e NotSupportedError) Unwrap() error {
	return ErrNotSupported
}
