package security

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

type BearerAuth struct {
	Enabled bool
	Token   string
}

func (a BearerAuth) Authorize(r *http.Request) bool {
	if !a.Enabled {
		return true
	}
	head := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(head, prefix) {
		return false
	}
	candidate := strings.TrimSpace(strings.TrimPrefix(head, prefix))
	if len(candidate) != len(a.Token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(a.Token)) == 1
}
