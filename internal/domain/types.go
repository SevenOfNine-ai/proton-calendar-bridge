package domain

import "time"

type Calendar struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ReadOnly    bool     `json:"read_only"`
	Shared      bool     `json:"shared"`
	Permissions []string `json:"permissions,omitempty"`
}

type Event struct {
	ID          string     `json:"id"`
	CalendarID  string     `json:"calendar_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Location    string     `json:"location,omitempty"`
	Start       time.Time  `json:"start"`
	End         time.Time  `json:"end"`
	AllDay      bool       `json:"all_day"`
	Recurrence  string     `json:"recurrence,omitempty"`
	Attendees   []string   `json:"attendees,omitempty"`
	Reminders   []string   `json:"reminders,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type EventMutation struct {
	CalendarID  string    `json:"calendar_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Recurrence  string    `json:"recurrence,omitempty"`
	Attendees   []string  `json:"attendees,omitempty"`
	Reminders   []string  `json:"reminders,omitempty"`
}
