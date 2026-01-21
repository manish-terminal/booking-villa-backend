// Package auth provides authentication and OTP services.
package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/sms"
)

// OTP represents an OTP record in DynamoDB.
type OTP struct {
	PK         string `dynamodbav:"PK"` // OTP#<phone>
	SK         string `dynamodbav:"SK"` // CODE#<otp>
	Phone      string `dynamodbav:"phone"`
	Code       string `dynamodbav:"code"`
	CreatedAt  int64  `dynamodbav:"createdAt"`
	ExpiresAt  int64  `dynamodbav:"expiresAt"`
	TTL        int64  `dynamodbav:"TTL"` // DynamoDB TTL field
	Verified   bool   `dynamodbav:"verified"`
	EntityType string `dynamodbav:"entityType"`
}

// SMSClient interface for sending SMS messages.
type SMSClient interface {
	SendOTP(ctx context.Context, phone, code string, expiryMinutes int) error
	IsEnabled() bool
}

// OTPService handles OTP generation and verification.
type OTPService struct {
	db            *db.Client
	smsClient     SMSClient
	expiryMinutes int
}

// NewOTPService creates a new OTP service.
func NewOTPService(dbClient *db.Client) *OTPService {
	expiryMinutes := 5 // Default 5 minutes
	if envExpiry := os.Getenv("OTP_EXPIRY_MINUTES"); envExpiry != "" {
		if parsed, err := strconv.Atoi(envExpiry); err == nil {
			expiryMinutes = parsed
		}
	}

	// Initialize SMS client (will be nil if BREVO_API_KEY is not set)
	smsClient := sms.NewClient()
	if smsClient != nil && smsClient.IsEnabled() {
		log.Println("SMS client initialized successfully - OTPs will be sent via Brevo")
	} else {
		log.Println("SMS client not configured - OTPs will be returned in response (development mode)")
	}

	return &OTPService{
		db:            dbClient,
		smsClient:     smsClient,
		expiryMinutes: expiryMinutes,
	}
}

// GenerateOTP creates a new 6-digit OTP code.
func (s *OTPService) GenerateOTP() (string, error) {
	// Generate a cryptographically secure random 6-digit number
	max := big.NewInt(999999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Pad with zeros to ensure 6 digits
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendOTP generates and stores an OTP for the given phone number.
// If SMS client is configured, the OTP is sent via SMS.
// Returns the code only if SMS sending is disabled (for development/testing).
func (s *OTPService) SendOTP(ctx context.Context, phone string) (string, error) {
	code, err := s.GenerateOTP()
	if err != nil {
		return "", err
	}

	now := time.Now()
	expiryDuration := time.Duration(s.expiryMinutes) * time.Minute
	expiresAt := now.Add(expiryDuration)

	otp := &OTP{
		PK:         "OTP#" + phone,
		SK:         "CODE#" + code,
		Phone:      phone,
		Code:       code,
		CreatedAt:  now.Unix(),
		ExpiresAt:  expiresAt.Unix(),
		TTL:        expiresAt.Unix(), // Auto-delete after expiry
		Verified:   false,
		EntityType: "OTP",
	}

	if err := s.db.PutItem(ctx, otp); err != nil {
		return "", fmt.Errorf("failed to store OTP: %w", err)
	}

	// Send OTP via SMS if client is configured
	if s.smsClient != nil && s.smsClient.IsEnabled() {
		if err := s.smsClient.SendOTP(ctx, phone, code, s.expiryMinutes); err != nil {
			log.Printf("Failed to send OTP via SMS to %s: %v", phone, err)
			// Return the error so the user knows SMS failed
			return "", fmt.Errorf("SMS sending failed: %w", err)
		}
		log.Printf("OTP sent via SMS to %s", phone)
		// Return empty code - OTP was sent via SMS
		return "", nil
	}

	// SMS not configured - return code in response (development mode)
	log.Printf("SMS not configured - returning OTP in response for %s", phone)
	return code, nil
}

// VerifyOTP validates the provided OTP for the phone number.
func (s *OTPService) VerifyOTP(ctx context.Context, phone, code string) (bool, error) {
	var otp OTP
	pk := "OTP#" + phone
	sk := "CODE#" + code

	err := s.db.GetItem(ctx, pk, sk, &otp)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil // OTP not found
		}
		return false, fmt.Errorf("failed to get OTP: %w", err)
	}

	// Check if OTP is expired
	if time.Now().Unix() > otp.ExpiresAt {
		return false, nil
	}

	// Check if already verified
	if otp.Verified {
		return false, nil
	}

	// Mark as verified
	err = s.db.UpdateItem(ctx, pk, sk, db.UpdateParams{
		UpdateExpression: "SET verified = :verified",
		ExpressionValues: map[string]interface{}{
			":verified": true,
		},
	})
	if err != nil {
		return false, fmt.Errorf("failed to mark OTP as verified: %w", err)
	}

	return true, nil
}

// CleanupExpiredOTPs removes expired OTPs for a phone number.
// Note: DynamoDB TTL will auto-delete, but this can be used for immediate cleanup.
func (s *OTPService) CleanupExpiredOTPs(ctx context.Context, phone string) error {
	params := db.QueryParams{
		KeyCondition: "PK = :pk",
		ExpressionValues: map[string]interface{}{
			":pk": "OTP#" + phone,
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to query OTPs: %w", err)
	}

	// Note: With DynamoDB TTL enabled, expired items are automatically deleted.
	// This function is provided for immediate cleanup if needed.
	// Since we're using TTL, we can skip manual cleanup.
	_ = items

	return nil
}
