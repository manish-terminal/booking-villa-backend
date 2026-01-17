// Package bookings provides booking management services.
package bookings

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/db"
	"github.com/google/uuid"
)

// BookingStatus represents the status of a booking.
type BookingStatus string

const (
	StatusPendingConfirmation BookingStatus = "pending_confirmation"
	StatusConfirmed           BookingStatus = "confirmed"
	StatusCheckedIn           BookingStatus = "checked_in"
	StatusCheckedOut          BookingStatus = "checked_out"
	StatusCancelled           BookingStatus = "cancelled"
	StatusNoShow              BookingStatus = "no_show"
)

// IsValid checks if the booking status is valid.
func (s BookingStatus) IsValid() bool {
	switch s {
	case StatusPendingConfirmation, StatusConfirmed, StatusCheckedIn,
		StatusCheckedOut, StatusCancelled, StatusNoShow:
		return true
	}
	return false
}

// Booking represents a property booking.
type Booking struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // BOOKING#<id>
	SK string `dynamodbav:"SK"` // METADATA

	// GSI1 for querying by property and date
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // PROPERTY#<propertyId>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // DATE#<checkInDate>

	// Booking fields
	ID           string `dynamodbav:"id" json:"id"`
	PropertyID   string `dynamodbav:"propertyId" json:"propertyId"`
	PropertyName string `dynamodbav:"propertyName,omitempty" json:"propertyName,omitempty"`

	// Guest information
	GuestName  string `dynamodbav:"guestName" json:"guestName"`
	GuestPhone string `dynamodbav:"guestPhone" json:"guestPhone"`
	GuestEmail string `dynamodbav:"guestEmail,omitempty" json:"guestEmail,omitempty"`
	NumGuests  int    `dynamodbav:"numGuests" json:"numGuests"`

	// Booking details
	CheckIn   time.Time `dynamodbav:"checkIn" json:"checkIn"`
	CheckOut  time.Time `dynamodbav:"checkOut" json:"checkOut"`
	NumNights int       `dynamodbav:"numNights" json:"numNights"`

	// Pricing
	PricePerNight float64 `dynamodbav:"pricePerNight" json:"pricePerNight"`
	TotalAmount   float64 `dynamodbav:"totalAmount" json:"totalAmount"`
	Currency      string  `dynamodbav:"currency" json:"currency"`

	// Status
	Status BookingStatus `dynamodbav:"status" json:"status"`

	// Agent/booking source
	BookedBy     string `dynamodbav:"bookedBy" json:"bookedBy"` // Phone of agent who made booking
	BookedByName string `dynamodbav:"bookedByName,omitempty" json:"bookedByName,omitempty"`
	InviteCode   string `dynamodbav:"inviteCode,omitempty" json:"inviteCode,omitempty"`

	// Notes
	Notes           string `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	SpecialRequests string `dynamodbav:"specialRequests,omitempty" json:"specialRequests,omitempty"`

	// Metadata
	CreatedAt  time.Time `dynamodbav:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time `dynamodbav:"updatedAt" json:"updatedAt"`
	EntityType string    `dynamodbav:"entityType" json:"-"`
}

// Service provides booking-related operations.
type Service struct {
	db *db.Client
}

// NewService creates a new booking service.
func NewService(dbClient *db.Client) *Service {
	return &Service{db: dbClient}
}

// CreateBooking creates a new booking.
func (s *Service) CreateBooking(ctx context.Context, booking *Booking) error {
	if booking.ID == "" {
		booking.ID = uuid.New().String()
	}

	now := time.Now()
	booking.PK = "BOOKING#" + booking.ID
	booking.SK = "METADATA"
	booking.GSI1PK = "PROPERTY#" + booking.PropertyID
	booking.GSI1SK = "DATE#" + booking.CheckIn.Format("2006-01-02")
	booking.CreatedAt = now
	booking.UpdatedAt = now
	booking.EntityType = "BOOKING"

	// Calculate number of nights if not set
	if booking.NumNights == 0 {
		booking.NumNights = int(booking.CheckOut.Sub(booking.CheckIn).Hours() / 24)
	}

	// Calculate total amount if not set
	if booking.TotalAmount == 0 && booking.PricePerNight > 0 {
		booking.TotalAmount = booking.PricePerNight * float64(booking.NumNights)
	}

	// Set default status
	if booking.Status == "" {
		booking.Status = StatusPendingConfirmation
	}

	// Set default currency
	if booking.Currency == "" {
		booking.Currency = "INR"
	}

	return s.db.PutItem(ctx, booking)
}

// GetBooking retrieves a booking by ID.
func (s *Service) GetBooking(ctx context.Context, id string) (*Booking, error) {
	var booking Booking
	pk := "BOOKING#" + id
	sk := "METADATA"

	err := s.db.GetItem(ctx, pk, sk, &booking)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get booking: %w", err)
	}

	return &booking, nil
}

// UpdateBooking updates an existing booking.
func (s *Service) UpdateBooking(ctx context.Context, booking *Booking) error {
	booking.UpdatedAt = time.Now()
	booking.PK = "BOOKING#" + booking.ID
	booking.SK = "METADATA"
	return s.db.PutItem(ctx, booking)
}

// UpdateBookingStatus updates only the status of a booking.
func (s *Service) UpdateBookingStatus(ctx context.Context, id string, status BookingStatus) error {
	pk := "BOOKING#" + id
	sk := "METADATA"
	now := time.Now().Format(time.RFC3339)

	params := db.UpdateParams{
		UpdateExpression: "SET #status = :status, updatedAt = :updatedAt",
		ExpressionValues: map[string]string{
			":status":    string(status),
			":updatedAt": now,
		},
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}

// DateRange represents a date range for queries.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// ListBookingsByProperty retrieves bookings for a property within a date range.
func (s *Service) ListBookingsByProperty(ctx context.Context, propertyID string, dateRange *DateRange) ([]*Booking, error) {
	keyCondition := "GSI1PK = :gsi1pk"
	expressionValues := map[string]string{
		":gsi1pk": "PROPERTY#" + propertyID,
	}

	// Add date range filter if provided
	if dateRange != nil {
		keyCondition += " AND GSI1SK BETWEEN :startDate AND :endDate"
		expressionValues[":startDate"] = "DATE#" + dateRange.Start.Format("2006-01-02")
		expressionValues[":endDate"] = "DATE#" + dateRange.End.Format("2006-01-02")
	}

	params := db.QueryParams{
		IndexName:        "GSI1",
		KeyCondition:     keyCondition,
		ExpressionValues: expressionValues,
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list bookings: %w", err)
	}

	bookings := make([]*Booking, 0, len(items))
	for _, item := range items {
		var booking Booking
		if err := attributevalue.UnmarshalMap(item, &booking); err != nil {
			return nil, fmt.Errorf("failed to unmarshal booking: %w", err)
		}
		bookings = append(bookings, &booking)
	}

	return bookings, nil
}

// ListBookingsByAgent retrieves bookings made by a specific agent.
func (s *Service) ListBookingsByAgent(ctx context.Context, agentPhone string) ([]*Booking, error) {
	// Note: This would benefit from a GSI on bookedBy
	// For now, we'll use a scan with filter (not ideal for production scale)
	// Consider adding GSI2 with GSI2PK = AGENT#<phone> for better performance

	// As a workaround, we can query the main table if we store agent bookings differently
	// For MVP, returning empty as this needs proper GSI
	return []*Booking{}, nil
}

// CheckAvailability checks if a property is available for the given dates.
func (s *Service) CheckAvailability(ctx context.Context, propertyID string, checkIn, checkOut time.Time) (bool, error) {
	// Get all bookings for the property in the date range
	dateRange := &DateRange{
		Start: checkIn.AddDate(0, 0, -1), // Include day before to catch overlaps
		End:   checkOut.AddDate(0, 0, 1), // Include day after to catch overlaps
	}

	bookings, err := s.ListBookingsByProperty(ctx, propertyID, dateRange)
	if err != nil {
		return false, err
	}

	// Check for overlapping bookings
	for _, booking := range bookings {
		// Skip cancelled or no-show bookings
		if booking.Status == StatusCancelled || booking.Status == StatusNoShow {
			continue
		}

		// Check if dates overlap
		// Overlap if: checkIn < existing.CheckOut AND checkOut > existing.CheckIn
		if checkIn.Before(booking.CheckOut) && checkOut.After(booking.CheckIn) {
			return false, nil
		}
	}

	return true, nil
}

// CancelBooking cancels a booking.
func (s *Service) CancelBooking(ctx context.Context, id string) error {
	return s.UpdateBookingStatus(ctx, id, StatusCancelled)
}

// ConfirmBooking confirms a pending booking.
func (s *Service) ConfirmBooking(ctx context.Context, id string) error {
	return s.UpdateBookingStatus(ctx, id, StatusConfirmed)
}

// CheckInBooking marks a booking as checked in.
func (s *Service) CheckInBooking(ctx context.Context, id string) error {
	return s.UpdateBookingStatus(ctx, id, StatusCheckedIn)
}

// CheckOutBooking marks a booking as checked out.
func (s *Service) CheckOutBooking(ctx context.Context, id string) error {
	return s.UpdateBookingStatus(ctx, id, StatusCheckedOut)
}
