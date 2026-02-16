package protonapi

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/sevenofnine/proton-calendar-bridge/internal/version"
)

const DefaultBaseURL = proton.DefaultHostURL

type ConnectionStatus string

const (
	StatusUnknown      ConnectionStatus = "unknown"
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
)

type ManagerAPI interface {
	NewClientWithLogin(ctx context.Context, username string, password []byte) (*proton.Client, proton.Auth, error)
	NewClientWithRefresh(ctx context.Context, uid, ref string) (*proton.Client, proton.Auth, error)
	NewClient(uid, acc, ref string) *proton.Client
}

type CalendarAPI interface {
	Auth2FA(ctx context.Context, req proton.Auth2FAReq) error
	GetCalendars(ctx context.Context) ([]proton.Calendar, error)
	GetCalendar(ctx context.Context, calendarID string) (proton.Calendar, error)
	GetCalendarKeys(ctx context.Context, calendarID string) (proton.CalendarKeys, error)
	GetCalendarMembers(ctx context.Context, calendarID string) ([]proton.CalendarMember, error)
	GetCalendarPassphrase(ctx context.Context, calendarID string) (proton.CalendarPassphrase, error)
	GetCalendarEvents(ctx context.Context, calendarID string, page, pageSize int, filter url.Values) ([]proton.CalendarEvent, error)
	GetCalendarEvent(ctx context.Context, calendarID, eventID string) (proton.CalendarEvent, error)
	GetAddresses(ctx context.Context) ([]proton.Address, error)
}

type Client struct {
	manager ManagerAPI
	client  CalendarAPI
	appVer  string

	mu     sync.RWMutex
	auth   Auth
	status ConnectionStatus
}

type ClientOptions struct {
	BaseURL      string
	AppVersion   string
	UID          string
	AccessToken  string
	RefreshToken string
	Manager      ManagerAPI
}

func NewClient(opts ClientOptions) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	appVersion := opts.AppVersion
	if appVersion == "" {
		appVersion = version.Version
	}
	manager := opts.Manager
	if manager == nil {
		manager = proton.New(
			proton.WithHostURL(baseURL),
			proton.WithAppVersion(fmt.Sprintf("proton-calendar-bridge@%s", appVersion)),
		)
	}
	c := &Client{
		manager: manager,
		appVer:  fmt.Sprintf("proton-calendar-bridge@%s", appVersion),
		status:  StatusUnknown,
	}
	if opts.UID != "" && opts.AccessToken != "" {
		c.client = manager.NewClient(opts.UID, opts.AccessToken, opts.RefreshToken)
		c.auth = Auth{UID: opts.UID, AccessToken: opts.AccessToken, RefreshToken: opts.RefreshToken}
	}
	return c
}

func (c *Client) SetSession(auth Auth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = auth
	if c.manager != nil {
		c.client = c.manager.NewClient(auth.UID, auth.AccessToken, auth.RefreshToken)
	}
}

func (c *Client) Status() ConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Client) session() (CalendarAPI, Auth) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client, c.auth
}

func (c *Client) setStatus(status ConnectionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *Client) setClient(client CalendarAPI, auth Auth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client = client
	c.auth = auth
	c.status = StatusConnected
}

func (c *Client) requireClient() (CalendarAPI, error) {
	client, _ := c.session()
	if client == nil {
		return nil, fmt.Errorf("proton session is not initialized")
	}
	return client, nil
}
