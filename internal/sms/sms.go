// Package sms provides SMS sending functionality using the Brevo API.
package sms

import (
	"context"
	"fmt"
	"os"

	brevo "github.com/getbrevo/brevo-go/lib"
)

// Client is a Brevo SMS client for sending transactional SMS messages.
type Client struct {
	apiClient *brevo.APIClient
	sender    string
}

// NewClient creates a new Brevo SMS client.
// It reads the BREVO_API_KEY and BREVO_SMS_SENDER environment variables.
// Returns nil if BREVO_API_KEY is not set (SMS sending will be disabled).
func NewClient() *Client {
	apiKey := os.Getenv("BREVO_API_KEY")
	if apiKey == "" {
		return nil
	}

	sender := os.Getenv("BREVO_SMS_SENDER")
	if sender == "" {
		sender = "VillaBook" // Default sender name
	}

	cfg := brevo.NewConfiguration()
	cfg.AddDefaultHeader("api-key", apiKey)

	return &Client{
		apiClient: brevo.NewAPIClient(cfg),
		sender:    sender,
	}
}

// SendOTP sends an OTP code to the specified phone number.
// The phone number should include the country code (e.g., "+91XXXXXXXXXX").
func (c *Client) SendOTP(ctx context.Context, phone, code string, expiryMinutes int) error {
	if c == nil {
		return fmt.Errorf("SMS client not initialized")
	}

	// Format the message content
	content := fmt.Sprintf("Your verification code is: %s. Valid for %d minutes. Do not share this code with anyone.", code, expiryMinutes)

	smsRequest := brevo.SendTransacSms{
		Sender:    c.sender,
		Recipient: phone,
		Content:   content,
		Type_:     "transactional",
	}

	_, resp, err := c.apiClient.TransactionalSMSApi.SendTransacSms(ctx, smsRequest)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SMS API returned status %d", resp.StatusCode)
	}

	return nil
}

// IsEnabled returns true if the SMS client is properly configured and enabled.
func (c *Client) IsEnabled() bool {
	return c != nil && c.apiClient != nil
}
