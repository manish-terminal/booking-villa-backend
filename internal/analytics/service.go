// Package analytics provides analytics and reporting services.
package analytics

import (
	"context"
	"time"

	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/payments"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// OwnerAnalytics represents analytics data for property owners.
type OwnerAnalytics struct {
	// Owner Info
	OwnerName  string `json:"ownerName"`
	OwnerPhone string `json:"ownerPhone"`

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
	// Agent Info
	AgentName  string `json:"agentName"`
	AgentPhone string `json:"agentPhone"`

	// Summary
	TotalBookings     int     `json:"totalBookings"`
	TotalBookingValue float64 `json:"totalBookingValue"`
	TotalCollected    float64 `json:"totalCollected"`
	TotalCommission   float64 `json:"totalCommission"`
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
	BookingID       string    `json:"bookingId"`
	PropertyName    string    `json:"propertyName"`
	GuestName       string    `json:"guestName"`
	CheckIn         time.Time `json:"checkIn"`
	CheckOut        time.Time `json:"checkOut"`
	TotalAmount     float64   `json:"totalAmount"`
	AgentCommission float64   `json:"agentCommission,omitempty"`
	Status          string    `json:"status"`
	PaymentStatus   string    `json:"paymentStatus"`
}

// Service provides analytics operations.
type Service struct {
	db              *db.Client
	propertyService *properties.Service
	bookingService  *bookings.Service
	paymentService  *payments.Service
	userService     *users.Service
}

// NewService creates a new analytics service.
func NewService(dbClient *db.Client) *Service {
	return &Service{
		db:              dbClient,
		propertyService: properties.NewService(dbClient),
		bookingService:  bookings.NewService(dbClient),
		paymentService:  payments.NewService(dbClient),
		userService:     users.NewService(dbClient),
	}
}

// GetOwnerAnalytics retrieves analytics for a property owner.
func (s *Service) GetOwnerAnalytics(ctx context.Context, ownerID string, startDate, endDate time.Time) (*OwnerAnalytics, error) {
	analytics := &OwnerAnalytics{
		OwnerPhone:       ownerID,
		Currency:         "INR",
		BookingsByStatus: make(map[string]int),
		PaymentsByStatus: make(map[string]int),
		PropertyStats:    []PropertyStat{},
		PeriodStart:      startDate,
		PeriodEnd:        endDate,
	}

	// Get owner's profile
	user, err := s.userService.GetUserByPhone(ctx, ownerID)
	if err == nil && user != nil {
		analytics.OwnerName = user.Name
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
		AgentPhone:       agentPhone,
		Currency:         "INR",
		BookingsByStatus: make(map[string]int),
		RecentBookings:   []BookingSummary{},
		PeriodStart:      startDate,
		PeriodEnd:        endDate,
	}

	// 1. Get agent's profile to see managed properties
	user, err := s.userService.GetUserByPhone(ctx, agentPhone)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return analytics, nil
	}

	// Set agent name from user profile
	analytics.AgentName = user.Name

	dateRange := &bookings.DateRange{Start: startDate, End: endDate}

	// 2. Iterate through managed properties to fetch bookings
	for _, propID := range user.ManagedProperties {
		propBookings, err := s.bookingService.ListBookingsByProperty(ctx, propID, dateRange)
		if err != nil {
			continue
		}

		for _, booking := range propBookings {
			// Only include bookings made by this agent
			if booking.BookedBy != agentPhone {
				continue
			}

			analytics.TotalBookings++
			analytics.TotalBookingValue += booking.TotalAmount
			analytics.TotalCommission += booking.AgentCommission
			analytics.BookingsByStatus[string(booking.Status)]++

			// Get payment info
			paymentStatus := "pending"
			if summary, err := s.paymentService.CalculatePaymentStatus(ctx, booking.ID); err == nil {
				analytics.TotalCollected += summary.TotalPaid
				paymentStatus = string(summary.Status)
			}

			// Add to recent bookings (limit to 100 entries before sorting in future if needed)
			if len(analytics.RecentBookings) < 50 {
				analytics.RecentBookings = append(analytics.RecentBookings, BookingSummary{
					BookingID:       booking.ID,
					PropertyName:    booking.PropertyName,
					GuestName:       booking.GuestName,
					CheckIn:         booking.CheckIn,
					CheckOut:        booking.CheckOut,
					TotalAmount:     booking.TotalAmount,
					AgentCommission: booking.AgentCommission,
					Status:          string(booking.Status),
					PaymentStatus:   paymentStatus,
				})
			}
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
func (s *Service) GetDashboardStats(ctx context.Context, phone string) (*DashboardStats, error) {
	stats := &DashboardStats{
		Currency: "INR",
	}

	today := time.Now().Truncate(24 * time.Hour)

	// 1. Get properties the user OWNS
	props, err := s.propertyService.ListPropertiesByOwner(ctx, phone)
	if err != nil {
		props = []*properties.Property{}
	}

	// Create a map of property IDs to avoid duplicates
	propertyIDs := make(map[string]bool)
	for _, p := range props {
		propertyIDs[p.ID] = true
	}

	// 2. Get properties the user MANAGES (for agents)
	user, err := s.userService.GetUserByPhone(ctx, phone)
	if err == nil && user != nil {
		for _, propID := range user.ManagedProperties {
			propertyIDs[propID] = true
		}
	}

	// 3. Collect all relevant bookings for these properties
	// We use a broad range (past 30 days to future 60 days) to catch:
	// - Today's check-ins (check-in = today)
	// - Today's check-outs (check-in < today, but check-out = today)
	// - Pending approvals (usually upcoming or very recent)
	// - Pending payments (can be past or upcoming)
	dateRange := &bookings.DateRange{
		Start: today.AddDate(0, 0, -30),
		End:   today.AddDate(0, 0, 60),
	}

	for propID := range propertyIDs {
		propBookings, err := s.bookingService.ListBookingsByProperty(ctx, propID, dateRange)
		if err != nil {
			continue
		}

		for _, booking := range propBookings {
			// Today's Check-Ins
			if booking.CheckIn.Truncate(24 * time.Hour).Equal(today) {
				stats.TodayCheckIns++
			}

			// Today's Check-Outs
			if booking.CheckOut.Truncate(24 * time.Hour).Equal(today) {
				stats.TodayCheckOuts++
			}

			// Pending Approvals
			if booking.Status == bookings.StatusPending {
				stats.PendingApprovals++
			}

			// Pending Payments & Due Amount
			// We only count payments for bookings that are not cancelled
			if booking.Status != bookings.StatusCancelled {
				if summary, err := s.paymentService.CalculatePaymentStatus(ctx, booking.ID); err == nil {
					if summary.Status != payments.PaymentStatusSettled {
						stats.PendingPayments++
						stats.TotalDueAmount += summary.TotalDue
					}
				}
			}
		}
	}

	return stats, nil
}
