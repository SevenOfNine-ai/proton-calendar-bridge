package protonapi

import (
	"context"
	"errors"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
)

func TestAuthenticatorLoginAndRefresh(t *testing.T) {
	mgr := &fakeManager{
		loginClient:   &proton.Client{},
		loginAuth:     proton.Auth{UID: "uid", AccessToken: "a1", RefreshToken: "r1"},
		refreshClient: &proton.Client{},
		refreshAuth:   proton.Auth{UID: "uid", AccessToken: "a2", RefreshToken: "r2"},
	}
	c := NewClient(ClientOptions{Manager: mgr})
	a := NewAuthenticator(c)

	auth, err := a.Login(context.Background(), "user", "pass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if auth.AccessToken != "a1" {
		t.Fatalf("unexpected auth: %+v", auth)
	}
	auth, err = a.Refresh(context.Background(), "uid", "r1")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if auth.AccessToken != "a2" {
		t.Fatalf("unexpected refresh auth: %+v", auth)
	}
}

func TestAuthenticatorSubmit2FA(t *testing.T) {
	c := &Client{client: &fakeCalendarClient{}}
	a := NewAuthenticator(c)
	if err := a.Submit2FA(context.Background(), "123456"); err != nil {
		t.Fatalf("submit2fa: %v", err)
	}
}

func TestAuthenticatorErrors(t *testing.T) {
	mgr := &fakeManager{loginErr: errors.New("boom")}
	c := NewClient(ClientOptions{Manager: mgr})
	a := NewAuthenticator(c)
	if _, err := a.Login(context.Background(), "", ""); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := a.Login(context.Background(), "user", "pass"); err == nil {
		t.Fatal("expected login error")
	}
	if err := a.Submit2FA(context.Background(), ""); err == nil {
		t.Fatal("expected empty code error")
	}
}
