// Package analytics provides analytics and reporting services.
package analytics

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/payments"
	"github.com/booking-villa-backend/internal/properties"
)

// OwnerAnalytics represents analytics data for property owners.
type OwnerAnalytics struct {
	// Summary
	TotalProperties int     `json:"totalProperties"`
	TotalBookings   int     `json:"totalBookings"`
	TotalRevenue    float64 `json:"totalRevenue"`
	TotalCollected  float64 `json:"totalCollected"`
	TotalPending    float64 `json:"totalPending"`
	Currency        string  `json:"currency"`

	// Booking breakdown
	BookingsByStatus map[string]int `json:"bookingsByStatus"`

	// Payment breakdown
	PaymentsByStatus map[string]int `json:"paymentsByStatus"`

	// Property-wise stats
	PropertyStats []PropertyStat `json:"propertyStats"`

	// Period
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
}

// PropertyStat represents analytics for a single property.
type PropertyStat struct {
	PropertyID     string  `json:"propertyId"`
	PropertyName   string  `json:"propertyName"`
	TotalBookings  int     `json:"totalBookings"`
	TotalRevenue   float64 `json:"totalRevenue"`
	TotalCollected float64 `json:"totalCollected"`
	OccupancyDays  int     `json:"occupancyDays"`
}

// AgentAnalytics represents analytics data for agents.
type AgentAnalytics struct {
	// Summary
	TotalBookings     int     `json:"totalBookings"`
	TotalBookingValue float64 `json:"totalBookingValue"`
	TotalCollected    float64 `json:"totalCollected"`
	Currency          string  `json:"currency"`

	// Booking breakdown
	BookingsByStatus map[string]int `json:"bookingsByStatus"`

	// Recent bookings
	RecentBookings []BookingSummary `json:"recentBookings"`

	// Period
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
}

// BookingSummary is a condensed booking for analytics.
type BookingSummary struct {
	BookingID     string    `json:"bookingId"`
	PropertyName  string    `json:"propertyName"`
	GuestName     string    `json:"guestName"`
	CheckIn       time.Time `json:"checkIn"`
	CheckOut      time.Time `json:"checkOut"`
	TotalAmount   float64   `json:"totalAmount"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"paymentStatus"`
}

// Service provides analytics operations.
type Service struct {
	db              *db.Client
	propertyService *properties.Service
	bookingService  *bookings.Service
	paymentService  *payments.Service
}

// NewService creates a new analytics service.
func NewService(dbClient *db.Client) *Service {
	return &Service{
		db:              dbClient,
		propertyService: properties.NewService(dbClient),
		bookingService:  bookings.NewService(dbClient),
		paymentService:  payments.NewService(dbClient),
	}
}

// GetOwnerAnalytics retrieves analytics for a property owner.
func (s *Service) GetOwnerAnalytics(ctx context.Context, ownerID string, startDate, endDate time.Time) (*OwnerAnalytics, error) {
	analytics := &OwnerAnalytics{
		Currency:         "INR",
		BookingsByStatus: make(map[string]int),
		PaymentsByStatus: make(map[string]int),
		PropertyStats:    []PropertyStat{},
		PeriodStart:      startDate,
		PeriodEnd:        endDate,
	}

	// Get owner's properties
	props, err := s.propertyService.ListPropertiesByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	analytics.TotalProperties = len(props)

	// Get bookings and payments for each property
	dateRange := &bookings.DateRange{Start: startDate, End: endDate}

	for _, prop := range props {
		propStat := PropertyStat{
			PropertyID:   prop.ID,
			PropertyName: prop.Name,
		}

		// Get bookings for this property
		propBookings, err := s.bookingService.ListBookingsByProperty(ctx, prop.ID, dateRange)
		if err != nil {
			continue
		}

		for _, booking := range propBookings {
			propStat.TotalBookings++
			propStat.TotalRevenue += booking.TotalAmount
			propStat.OccupancyDays += booking.NumNights

			analytics.TotalBookings++
			analytics.TotalRevenue += booking.TotalAmount
			analytics.BookingsByStatus[string(booking.Status)]++

			// Get payment status for this booking
			paymentSummary, err := s.paymentService.CalculatePaymentStatus(ctx, booking.ID)
			if err == nil {
				propStat.TotalCollected += paymentSummary.TotalPaid
				analytics.TotalCollected += paymentSummary.TotalPaid
				analytics.PaymentsByStatus[string(paymentSummary.Status)]++
			}
		}

		analytics.PropertyStats = append(analytics.PropertyStats, propStat)
	}

	analytics.TotalPending = analytics.TotalRevenue - analytics.TotalCollected

	return analytics, nil
}

// GetAgentAnalytics retrieves analytics for an agent.
func (s *Service) GetAgentAnalytics(ctx context.Context, agentPhone string, startDate, endDate time.Time) (*AgentAnalytics, error) {
	analytics := &AgentAnalytics{
		Currency:         "INR",
		BookingsByStatus: make(map[string]int),
		RecentBookings:   []BookingSummary{},
		PeriodStart:      startDate,
		PeriodEnd:        endDate,
	}

	// Query bookings made by this agent
	// Note: For production, consider adding a GSI on bookedBy field
	// Current approach: Try to query from GSI if available, otherwise return empty
	// Add GSI2 with PK=AGENT#<phone> for better performance in production

	allBookingsParams := db.QueryParams{
		KeyCondition: "GSI1PK = :gsi1pk",
		ExpressionValues: map[string]string{
			":gsi1pk": "AGENT#" + agentPhone,
		},
		IndexName: "GSI1",
	}

	items, err := s.db.Query(ctx, allBookingsParams)
	if err != nil {
		// Fallback: return empty analytics if no GSI
		return analytics, nil
	}

	for _, item := range items {
		var booking bookings.Booking
		if err := attributevalue.UnmarshalMap(item, &booking); err != nil {
			continue
		}

		// Filter by date range
		if booking.CheckIn.Before(startDate) || booking.CheckIn.After(endDate) {
			continue
		}

		analytics.TotalBookings++
		analytics.TotalBookingValue += booking.TotalAmount
		analytics.BookingsByStatus[string(booking.Status)]++

		// Get payment info
		paymentStatus := "pending"
		if summary, err := s.paymentService.CalculatePaymentStatus(ctx, booking.ID); err == nil {
			analytics.TotalCollected += summary.TotalPaid
			paymentStatus = string(summary.Status)
		}

		// Add to recent bookings (limit to 10)
		if len(analytics.RecentBookings) < 10 {
			analytics.RecentBookings = append(analytics.RecentBookings, BookingSummary{
				BookingID:     booking.ID,
				PropertyName:  booking.PropertyName,
				GuestName:     booking.GuestName,
				CheckIn:       booking.CheckIn,
				CheckOut:      booking.CheckOut,
				TotalAmount:   booking.TotalAmount,
				Status:        string(booking.Status),
				PaymentStatus: paymentStatus,
			})
		}
	}

	return analytics, nil
}

// GetDashboardStats returns quick stats for dashboard.
type DashboardStats struct {
	TodayCheckIns    int     `json:"todayCheckIns"`
	TodayCheckOuts   int     `json:"todayCheckOuts"`
	PendingApprovals int     `json:"pendingApprovals"`
	PendingPayments  int     `json:"pendingPayments"`
	TotalDueAmount   float64 `json:"totalDueAmount"`
	Currency         string  `json:"currency"`
}

// GetDashboardStats retrieves quick dashboard stats.
func (s *Service) GetDashboardStats(ctx context.Context, ownerID string) (*DashboardStats, error) {
	stats := &DashboardStats{
		Currency: "INR",
	}

	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.AddDate(0, 0, 1)

	// Get owner's properties
	props, err := s.propertyService.ListPropertiesByOwner(ctx, ownerID)
	if err != nil {
		return stats, nil
	}

	for _, prop := range props {
		// Get today's bookings
		dateRange := &bookings.DateRange{Start: today, End: tomorrow}
		propBookings, err := s.bookingService.ListBookingsByProperty(ctx, prop.ID, dateRange)
		if err != nil {
			continue
		}

		for _, booking := range propBookings {
			if booking.CheckIn.Truncate(24 * time.Hour).Equal(today) {
				stats.TodayCheckIns++
			}
			if booking.CheckOut.Truncate(24 * time.Hour).Equal(today) {
				stats.TodayCheckOuts++
			}
			if booking.Status == bookings.StatusPendingConfirmation {
				stats.PendingApprovals++
			}

			// Check payment status
			if summary, err := s.paymentService.CalculatePaymentStatus(ctx, booking.ID); err == nil {
				if summary.Status != payments.PaymentStatusCompleted {
					stats.PendingPayments++
					stats.TotalDueAmount += summary.TotalDue
				}
			}
		}
	}

	return stats, nil
}
