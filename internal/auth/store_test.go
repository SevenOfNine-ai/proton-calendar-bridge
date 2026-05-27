package auth

import (
	"os"
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
}

func TestStoreLoadWrongPassword(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.enc")
	store := Store{Path: path}
	if err := store.Save(Session{UID: "uid"}, "correct-password"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.Load("wrong-password"); err == nil {
		t.Fatal("expected decrypt error with wrong password")
	}
}

func TestStoreSaveEmptyPath(t *testing.T) {
	t.Parallel()
	store := Store{Path: ""}
	if err := store.Save(Session{UID: "uid"}, "password"); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestStoreLoadEmptyPath(t *testing.T) {
	t.Parallel()
	store := Store{Path: ""}
	if _, err := store.Load("password"); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestStoreLoadInvalidData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.enc")
	if err := os.WriteFile(path, []byte("too short"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	store := Store{Path: path}
	if _, err := store.Load("password"); err == nil {
		t.Fatal("expected error for invalid encrypted session")
	}
}

func TestStoreLoadCorruptedData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.enc")
	store := Store{Path: path}
	if err := store.Save(Session{UID: "uid"}, "password"); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Corrupt the file
	blob, _ := os.ReadFile(path)
	blob[0] ^= 0xff // flip a bit
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("corrupt file: %v", err)
	}
	if _, err := store.Load("password"); err == nil {
		t.Fatal("expected error for corrupted data")
	}
}

func TestStoreRoundTripEmptySession(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.enc")
	store := Store{Path: path}
	in := Session{UID: "", AccessToken: "", RefreshToken: "", Username: ""}
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
}
