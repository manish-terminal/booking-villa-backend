// Package payments provides offline payment tracking services.
package payments

import (
	"context"
	"fmt"
	"time"

	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
)

// PaymentStatus represents the overall payment status for a booking.
type PaymentStatus string

const (
	// PaymentStatusPending - No payments recorded yet
	PaymentStatusPending PaymentStatus = "pending"
	// PaymentStatusPartial - Partial payment received, amount still due
	PaymentStatusPartial PaymentStatus = "partial"
	// PaymentStatusSettled - Full payment received
	PaymentStatusSettled PaymentStatus = "settled"
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

// PaymentSummary provides an overview of payments for a booking.
type PaymentSummary struct {
	BookingID       string        `json:"bookingId"`
	TotalAmount     float64       `json:"totalAmount"`
	TotalPaid       float64       `json:"totalPaid"`
	TotalDue        float64       `json:"totalDue"`
	AgentCommission float64       `json:"agentCommission,omitempty"`
	Status          PaymentStatus `json:"status"`
	Currency        string        `json:"currency"`
	LastUpdated     time.Time     `json:"lastUpdated"`
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

// CalculatePaymentStatus computes the payment status for a booking based on AdvanceAmount.
func (s *Service) CalculatePaymentStatus(ctx context.Context, bookingID string) (*PaymentSummary, error) {
	// Get the booking to know the total amount and advance
	booking, err := s.bookingService.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking: %w", err)
	}

	if booking == nil {
		return nil, fmt.Errorf("booking not found")
	}

	totalPaid := booking.AdvanceAmount
	totalDue := booking.TotalAmount - totalPaid

	// Determine status
	var status PaymentStatus
	switch {
	case totalPaid <= 0:
		status = PaymentStatusPending
	case totalPaid >= booking.TotalAmount:
		status = PaymentStatusSettled
		totalDue = 0
	default:
		status = PaymentStatusPartial
	}

	return &PaymentSummary{
		BookingID:       bookingID,
		TotalAmount:     booking.TotalAmount,
		TotalPaid:       totalPaid,
		TotalDue:        totalDue,
		AgentCommission: booking.AgentCommission,
		Status:          status,
		Currency:        booking.Currency,
		LastUpdated:     booking.UpdatedAt,
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
