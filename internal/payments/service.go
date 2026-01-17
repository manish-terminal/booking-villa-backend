// Package payments provides offline payment tracking services.
package payments

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/google/uuid"
)

// PaymentStatus represents the overall payment status for a booking.
type PaymentStatus string

const (
	// PaymentStatusPending - No payments recorded yet
	PaymentStatusPending PaymentStatus = "pending"
	// PaymentStatusDue - Partial payment received, amount still due
	PaymentStatusDue PaymentStatus = "due"
	// PaymentStatusCompleted - Full payment received
	PaymentStatusCompleted PaymentStatus = "completed"
)

// PaymentMethod represents the method used for offline payment.
type PaymentMethod string

const (
	PaymentMethodCash   PaymentMethod = "cash"
	PaymentMethodUPI    PaymentMethod = "upi"
	PaymentMethodBank   PaymentMethod = "bank_transfer"
	PaymentMethodCheque PaymentMethod = "cheque"
	PaymentMethodOther  PaymentMethod = "other"
)

// IsValid checks if the payment method is valid.
func (m PaymentMethod) IsValid() bool {
	switch m {
	case PaymentMethodCash, PaymentMethodUPI, PaymentMethodBank, PaymentMethodCheque, PaymentMethodOther:
		return true
	}
	return false
}

// Payment represents an offline payment record.
type Payment struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // PAYMENT#<bookingId>
	SK string `dynamodbav:"SK"` // DATE#<date>#<paymentId>

	// GSI1 for querying by booking and status
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // BOOKING#<bookingId>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // STATUS#<status>

	// Payment fields
	ID        string        `dynamodbav:"id" json:"id"`
	BookingID string        `dynamodbav:"bookingId" json:"bookingId"`
	Amount    float64       `dynamodbav:"amount" json:"amount"`
	Currency  string        `dynamodbav:"currency" json:"currency"`
	Method    PaymentMethod `dynamodbav:"method" json:"method"`

	// Reference for tracking offline payments
	Reference string `dynamodbav:"reference,omitempty" json:"reference,omitempty"` // Receipt number, cheque number, etc.

	// Who recorded the payment
	RecordedBy     string `dynamodbav:"recordedBy" json:"recordedBy"`
	RecordedByName string `dynamodbav:"recordedByName,omitempty" json:"recordedByName,omitempty"`

	// Notes
	Notes string `dynamodbav:"notes,omitempty" json:"notes,omitempty"`

	// Date when the payment was actually received (may differ from recorded date)
	PaymentDate time.Time `dynamodbav:"paymentDate" json:"paymentDate"`

	// Metadata
	CreatedAt  time.Time `dynamodbav:"createdAt" json:"createdAt"`
	EntityType string    `dynamodbav:"entityType" json:"-"`
}

// PaymentSummary provides an overview of payments for a booking.
type PaymentSummary struct {
	BookingID       string        `json:"bookingId"`
	TotalAmount     float64       `json:"totalAmount"`
	TotalPaid       float64       `json:"totalPaid"`
	TotalDue        float64       `json:"totalDue"`
	Status          PaymentStatus `json:"status"`
	PaymentCount    int           `json:"paymentCount"`
	Currency        string        `json:"currency"`
	LastPaymentDate *time.Time    `json:"lastPaymentDate,omitempty"`
}

// Service provides payment-related operations.
type Service struct {
	db             *db.Client
	bookingService *bookings.Service
}

// NewService creates a new payment service.
func NewService(dbClient *db.Client) *Service {
	return &Service{
		db:             dbClient,
		bookingService: bookings.NewService(dbClient),
	}
}

// LogPayment records a new offline payment.
func (s *Service) LogPayment(ctx context.Context, payment *Payment) error {
	if payment.ID == "" {
		payment.ID = uuid.New().String()
	}

	now := time.Now()
	payment.PK = "PAYMENT#" + payment.BookingID
	payment.SK = "DATE#" + payment.PaymentDate.Format("2006-01-02") + "#" + payment.ID
	payment.GSI1PK = "BOOKING#" + payment.BookingID
	payment.GSI1SK = "PAYMENT#" + payment.ID
	payment.CreatedAt = now
	payment.EntityType = "PAYMENT"

	// Set default currency
	if payment.Currency == "" {
		payment.Currency = "INR"
	}

	// Set payment date to now if not specified
	if payment.PaymentDate.IsZero() {
		payment.PaymentDate = now
	}

	// Validate payment method
	if !payment.Method.IsValid() {
		payment.Method = PaymentMethodCash
	}

	return s.db.PutItem(ctx, payment)
}

// GetPaymentsByBooking retrieves all payments for a booking.
func (s *Service) GetPaymentsByBooking(ctx context.Context, bookingID string) ([]*Payment, error) {
	params := db.QueryParams{
		KeyCondition: "PK = :pk",
		ExpressionValues: map[string]string{
			":pk": "PAYMENT#" + bookingID,
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get payments: %w", err)
	}

	payments := make([]*Payment, 0, len(items))
	for _, item := range items {
		var payment Payment
		if err := attributevalue.UnmarshalMap(item, &payment); err != nil {
			continue // Skip invalid entries
		}
		payments = append(payments, &payment)
	}

	return payments, nil
}

// CalculatePaymentStatus computes the payment status for a booking.
// Returns the status and a summary of payments.
func (s *Service) CalculatePaymentStatus(ctx context.Context, bookingID string) (*PaymentSummary, error) {
	// Get the booking to know the total amount
	booking, err := s.bookingService.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking: %w", err)
	}

	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	// Get all payments for this booking
	payments, err := s.GetPaymentsByBooking(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payments: %w", err)
	}

	// Calculate totals
	var totalPaid float64
	var lastPaymentDate *time.Time

	for _, payment := range payments {
		totalPaid += payment.Amount
		if lastPaymentDate == nil || payment.PaymentDate.After(*lastPaymentDate) {
			lastPaymentDate = &payment.PaymentDate
		}
	}

	totalDue := booking.TotalAmount - totalPaid

	// Determine status
	var status PaymentStatus
	switch {
	case len(payments) == 0:
		status = PaymentStatusPending
	case totalPaid >= booking.TotalAmount:
		status = PaymentStatusCompleted
		totalDue = 0 // Ensure no negative due amount
	default:
		status = PaymentStatusDue
	}

	return &PaymentSummary{
		BookingID:       bookingID,
		TotalAmount:     booking.TotalAmount,
		TotalPaid:       totalPaid,
		TotalDue:        totalDue,
		Status:          status,
		PaymentCount:    len(payments),
		Currency:        booking.Currency,
		LastPaymentDate: lastPaymentDate,
	}, nil
}

// GetPaymentStatus returns just the payment status string.
func (s *Service) GetPaymentStatus(ctx context.Context, bookingID string) (PaymentStatus, error) {
	summary, err := s.CalculatePaymentStatus(ctx, bookingID)
	if err != nil {
		return "", err
	}
	return summary.Status, nil
}

// GetPayment retrieves a specific payment by ID.
func (s *Service) GetPayment(ctx context.Context, bookingID, paymentID string) (*Payment, error) {
	// We need to query since we don't know the exact date
	payments, err := s.GetPaymentsByBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	for _, payment := range payments {
		if payment.ID == paymentID {
			return payment, nil
		}
	}

	return nil, nil
}

// DeletePayment removes a payment record.
// Note: This should be used carefully and typically only by admins.
func (s *Service) DeletePayment(ctx context.Context, bookingID, paymentID string) error {
	// First, find the payment to get its exact SK
	payment, err := s.GetPayment(ctx, bookingID, paymentID)
	if err != nil {
		return err
	}

	if payment == nil {
		return fmt.Errorf("payment not found")
	}

	return s.db.DeleteItem(ctx, payment.PK, payment.SK)
}
