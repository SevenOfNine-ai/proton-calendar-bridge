package security

import (
	"net/http/httptest"
	"testing"
)

func TestAuthorize(t *testing.T) {
	a := BearerAuth{Enabled: true, Token: "abc123"}

	req := httptest.NewRequest("GET", "/", nil)
	if a.Authorize(req) {
		t.Fatal("expected false without header")
	}
	req.Header.Set("Authorization", "Bearer abc123")
	if !a.Authorize(req) {
		t.Fatal("expected authorized")
	}
	req.Header.Set("Authorization", "Bearer wrong")
	if a.Authorize(req) {
		t.Fatal("expected unauthorized")
	}
}

func TestAuthorizeDisabled(t *testing.T) {
	a := BearerAuth{Enabled: false, Token: "x"}
	req := httptest.NewRequest("GET", "/", nil)
	if !a.Authorize(req) {
		t.Fatal("expected auth bypass")
	}
}
