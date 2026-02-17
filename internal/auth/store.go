package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const saltSize = 16

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
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("salt: %w", err)
	}
	key := deriveKey(bridgePassword, salt)
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
	blob := append(append(salt, nonce...), ciphertext...)
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
	if len(blob) < saltSize {
		return Session{}, fmt.Errorf("invalid encrypted session")
	}
	salt := blob[:saltSize]
	key := deriveKey(bridgePassword, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Session{}, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Session{}, fmt.Errorf("gcm: %w", err)
	}
	if len(blob) < saltSize+gcm.NonceSize() {
		return Session{}, fmt.Errorf("invalid encrypted session")
	}
	nonce := blob[saltSize : saltSize+gcm.NonceSize()]
	ciphertext := blob[saltSize+gcm.NonceSize():]
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

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}
