package protonapi

import (
	"context"
	"fmt"

	proton "github.com/ProtonMail/go-proton-api"
)

type Authenticator struct {
	client *Client
}

func NewAuthenticator(client *Client) *Authenticator {
	return &Authenticator{client: client}
}

func (a *Authenticator) Login(ctx context.Context, username, password string) (Auth, error) {
	if username == "" || password == "" {
		return Auth{}, fmt.Errorf("username and password are required")
	}
	if a.client == nil || a.client.manager == nil {
		return Auth{}, fmt.Errorf("proton manager is not configured")
	}
	client, auth, err := a.client.manager.NewClientWithLogin(ctx, username, []byte(password))
	if err != nil {
		a.client.setStatus(StatusDisconnected)
		return Auth{}, err
	}
	a.client.setClient(client, auth)
	return auth, nil
}

func (a *Authenticator) Submit2FA(ctx context.Context, totpCode string) error {
	if totpCode == "" {
		return fmt.Errorf("totp code is required")
	}
	client, err := a.client.requireClient()
	if err != nil {
		return err
	}
	if err := client.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: totpCode}); err != nil {
		a.client.setStatus(StatusDisconnected)
		return err
	}
	a.client.setStatus(StatusConnected)
	return nil
}

func (a *Authenticator) Refresh(ctx context.Context, uid, refreshToken string) (Auth, error) {
	if uid == "" || refreshToken == "" {
		_, auth := a.client.session()
		uid = auth.UID
		refreshToken = auth.RefreshToken
	}
	if uid == "" || refreshToken == "" {
		return Auth{}, fmt.Errorf("uid and refresh token are required")
	}
	if a.client == nil || a.client.manager == nil {
		return Auth{}, fmt.Errorf("proton manager is not configured")
	}
	client, auth, err := a.client.manager.NewClientWithRefresh(ctx, uid, refreshToken)
	if err != nil {
		a.client.setStatus(StatusDisconnected)
		return Auth{}, err
	}
	a.client.setClient(client, auth)
	return auth, nil
}
