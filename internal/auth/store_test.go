package auth

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.enc")
	store := Store{Path: path}
	in := Session{UID: "uid", AccessToken: "at", RefreshToken: "rt", Username: "user"}
	if err := store.Save(in, "bridge-password"); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := store.Load("bridge-password")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch: got %+v want %+v", out, in)
	}
	if _, err := store.Load("wrong-password"); err == nil {
		t.Fatal("expected decrypt error with wrong password")
	}
}
