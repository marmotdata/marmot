package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidKey        = errors.New("invalid encryption key")
)

// Encryptor provides encryption/decryption operations using XChaCha20-Poly1305
type Encryptor struct {
	aead cipher.AEAD
}

// NewEncryptor creates a new encryptor with the given 32-byte key
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("%w: key must be %d bytes", ErrInvalidKey, chacha20poly1305.KeySize)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	return &Encryptor{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext with nonce prepended
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	// Generate random nonce
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)

	// Encode to base64 for safe storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// EncryptString encrypts a string value
func (e *Encryptor) EncryptString(plaintext string) (string, error) {
	return e.Encrypt([]byte(plaintext))
}

// Decrypt decrypts base64-encoded ciphertext (with nonce prepended)
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decoding ciphertext: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := e.aead.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt
	plaintext, err := e.aead.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

// DecryptString decrypts a base64-encoded ciphertext to a string
func (e *Encryptor) DecryptString(ciphertext string) (string, error) {
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptMap encrypts all string values in a map that match the given keys
func (e *Encryptor) EncryptMap(data map[string]interface{}, sensitiveKeys map[string]bool) error {
	for key, value := range data {
		if !sensitiveKeys[key] {
			continue
		}

		// Only encrypt string values
		strValue, ok := value.(string)
		if !ok || strValue == "" {
			continue
		}

		encrypted, err := e.EncryptString(strValue)
		if err != nil {
			return fmt.Errorf("encrypting field %s: %w", key, err)
		}

		data[key] = encrypted
	}

	return nil
}

// DecryptMap decrypts all string values in a map that match the given keys
func (e *Encryptor) DecryptMap(data map[string]interface{}, sensitiveKeys map[string]bool) error {
	for key, value := range data {
		if !sensitiveKeys[key] {
			continue
		}

		// Only decrypt string values
		strValue, ok := value.(string)
		if !ok || strValue == "" {
			continue
		}

		decrypted, err := e.DecryptString(strValue)
		if err != nil {
			return fmt.Errorf("decrypting field %s: %w", key, err)
		}

		data[key] = decrypted
	}

	return nil
}

// GenerateKey generates a random 32-byte encryption key
func GenerateKey() ([]byte, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	return key, nil
}

// EncodeKey encodes a key to base64 for storage
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64-encoded key
func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("%w: decoded key must be %d bytes", ErrInvalidKey, chacha20poly1305.KeySize)
	}
	return key, nil
}
