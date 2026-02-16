package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Session struct {
	UID          string `json:"uid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Username     string `json:"username"`
}

type Store struct {
	Path string
}

func (s Store) Save(session Session, bridgePassword string) error {
	if s.Path == "" {
		return fmt.Errorf("store path is required")
	}
	plaintext, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	key := deriveKey(bridgePassword)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	blob := append(nonce, ciphertext...)
	if err := os.WriteFile(s.Path, blob, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func (s Store) Load(bridgePassword string) (Session, error) {
	if s.Path == "" {
		return Session{}, fmt.Errorf("store path is required")
	}
	blob, err := os.ReadFile(s.Path)
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	key := deriveKey(bridgePassword)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Session{}, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Session{}, fmt.Errorf("gcm: %w", err)
	}
	if len(blob) < gcm.NonceSize() {
		return Session{}, fmt.Errorf("invalid encrypted session")
	}
	nonce := blob[:gcm.NonceSize()]
	ciphertext := blob[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return session, nil
}

func deriveKey(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return sum[:]
}
