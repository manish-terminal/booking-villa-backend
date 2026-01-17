// Package utils provides utility functions for JWT handling and password hashing.
package utils

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the custom claims embedded in JWT tokens.
type TokenClaims struct {
	UserID string `json:"userId"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// DefaultJWTConfig returns the default JWT configuration from environment.
func DefaultJWTConfig() JWTConfig {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-secret-change-in-production"
	}

	return JWTConfig{
		Secret:     secret,
		Expiration: 24 * time.Hour, // Default 24 hours
	}
}

// GenerateToken creates a new JWT token for a user.
func GenerateToken(userID, phone, role string) (string, error) {
	config := DefaultJWTConfig()

	claims := TokenClaims{
		UserID: userID,
		Phone:  phone,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.Expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "booking-villa-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// GenerateTokenWithExpiration creates a JWT token with custom expiration.
func GenerateTokenWithExpiration(userID, phone, role string, expiration time.Duration) (string, error) {
	config := DefaultJWTConfig()

	claims := TokenClaims{
		UserID: userID,
		Phone:  phone,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "booking-villa-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// ValidateToken validates a JWT token and returns the claims.
func ValidateToken(tokenString string) (*TokenClaims, error) {
	config := DefaultJWTConfig()

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// RefreshToken generates a new token from existing valid claims.
func RefreshToken(tokenString string) (string, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	// Generate new token with same user info but new expiration
	return GenerateToken(claims.UserID, claims.Phone, claims.Role)
}

// ExtractTokenFromHeader extracts the token from an Authorization header.
// Expected format: "Bearer <token>"
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", fmt.Errorf("invalid authorization header format")
	}
	return authHeader[7:], nil
}
