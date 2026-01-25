package analytics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/users"
)

// GenerateMasterCSV creates a CSV dump of all data.
func (s *Service) GenerateMasterCSV(ctx context.Context) ([]byte, error) {
	// 1. Fetch metadata (Properties and Agents)
	allProperties, err := s.propertyService.ListAllProperties(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch properties: %w", err)
	}

	allAgents, err := s.userService.ListUsersByRole(ctx, users.RoleAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agents: %w", err)
	}
	agentMap := make(map[string]string) // phone -> name
	for _, agent := range allAgents {
		agentMap[agent.Phone] = agent.Name
	}

	// 2. Fetch all bookings
	// We'll iterate all properties and get bookings for a wide range (e.g. past 2 years to future 1 year)
	now := time.Now()
	startDate := now.AddDate(-2, 0, 0)
	endDate := now.AddDate(1, 0, 0)
	dateRange := &bookings.DateRange{Start: startDate, End: endDate}

	var allBookings []*bookings.Booking

	for _, prop := range allProperties {
		propBookings, err := s.bookingService.ListBookingsByProperty(ctx, prop.ID, dateRange)
		if err == nil {
			allBookings = append(allBookings, propBookings...)
		}
	}

	// 3. Generate CSV
	var b bytes.Buffer
	w := csv.NewWriter(&b)

	// Header
	header := []string{
		"Booking ID", "Status", "Created At",
		"Property Name", "Property ID", "Owner Phone",
		"Guest Name", "Guest Phone", "Guest Email", "Num Guests",
		"Check In", "Check Out", "Nights",
		"Total Amount", "Agent Commission", "Currency",
		"Booked By Phone", "Booked By Name", "Invite Code",
		"Notes",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	// Rows
	for _, bk := range allBookings {
		// Resolve Agent Name
		agentName := "Direct/Owner"
		if bk.BookedBy != "" {
			if name, ok := agentMap[bk.BookedBy]; ok {
				agentName = name
			} else if bk.BookedByName != "" {
				agentName = bk.BookedByName
			} else {
				agentName = "Unknown Agent"
			}
		}

		// Resolve Property Owner
		ownerPhone := "Unknown"
		for _, p := range allProperties {
			if p.ID == bk.PropertyID {
				ownerPhone = p.OwnerID
				break
			}
		}

		row := []string{
			bk.ID, string(bk.Status), bk.CreatedAt.Format(time.RFC3339),
			bk.PropertyName, bk.PropertyID, ownerPhone,
			bk.GuestName, bk.GuestPhone, bk.GuestEmail, strconv.Itoa(bk.NumGuests),
			bk.CheckIn.Format("2006-01-02"), bk.CheckOut.Format("2006-01-02"), strconv.Itoa(bk.NumNights),
			fmt.Sprintf("%.2f", bk.TotalAmount), fmt.Sprintf("%.2f", bk.AgentCommission), bk.Currency,
			bk.BookedBy, agentName, bk.InviteCode,
			bk.Notes,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return b.Bytes(), w.Error()
}
