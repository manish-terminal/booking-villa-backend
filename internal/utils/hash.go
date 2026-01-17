package utils

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost is the bcrypt cost factor for password hashing.
	DefaultCost = 12
)

// HashPassword creates a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(bytes), nil
}

// VerifyPassword compares a password with its bcrypt hash.
// Returns true if they match, false otherwise.
func VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// IsPasswordHashed checks if a string appears to be a bcrypt hash.
func IsPasswordHashed(s string) bool {
	// Bcrypt hashes start with $2a$, $2b$, or $2y$
	if len(s) < 4 {
		return false
	}
	return s[0] == '$' && s[1] == '2' && (s[2] == 'a' || s[2] == 'b' || s[2] == 'y') && s[3] == '$'
}
