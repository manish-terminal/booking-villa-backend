// Package sms provides SMS sending functionality using the Brevo API.
package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const brevoSMSEndpoint = "https://api.brevo.com/v3/transactionalSMS/send"

// Client is a Brevo SMS client for sending transactional SMS messages.
type Client struct {
	apiKey     string
	sender     string
	httpClient *http.Client
}

// smsRequest represents the request body for Brevo SMS API.
type smsRequest struct {
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	Tag       string `json:"tag,omitempty"`
}

// smsResponse represents the response from Brevo SMS API.
type smsResponse struct {
	MessageID int64  `json:"messageId,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
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

	return &Client{
		apiKey: apiKey,
		sender: sender,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendOTP sends an OTP code to the specified phone number.
// The phone number should include the country code (e.g., "91XXXXXXXXXX" for India).
func (c *Client) SendOTP(ctx context.Context, phone, code string, expiryMinutes int) error {
	if c == nil {
		return fmt.Errorf("SMS client not initialized")
	}

	// Format phone number - remove + prefix if present (Brevo expects without +)
	formattedPhone := strings.TrimPrefix(phone, "+")

	log.Printf("Sending OTP to phone: %s, sender: %s", formattedPhone, c.sender)

	// Format the message content
	content := fmt.Sprintf("Your verification code is: %s. Valid for %d minutes. Do not share this code with anyone.", code, expiryMinutes)

	reqBody := smsRequest{
		Sender:    c.sender,
		Recipient: formattedPhone,
		Content:   content,
		Type:      "transactional",
		Tag:       "otp",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("Brevo SMS request: %s", string(jsonBody))

	req, err := http.NewRequestWithContext(ctx, "POST", brevoSMSEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Brevo SMS response status: %d, body: %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode >= 400 {
		var errResp smsResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("SMS API error (%d): %s - %s", resp.StatusCode, errResp.Code, errResp.Message)
		}
		return fmt.Errorf("SMS API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var successResp smsResponse
	if err := json.Unmarshal(bodyBytes, &successResp); err == nil {
		log.Printf("OTP sent successfully to %s, messageId: %d", formattedPhone, successResp.MessageID)
	}

	return nil
}

// IsEnabled returns true if the SMS client is properly configured and enabled.
func (c *Client) IsEnabled() bool {
	return c != nil && c.apiKey != ""
}
