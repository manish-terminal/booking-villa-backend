package analytics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// GenerateMasterCSV creates a CSV dump of all data.
func (s *Service) GenerateMasterCSV(ctx context.Context) ([]byte, error) {
	// 1. Fetch ALL properties via Scan (robust for metadata lookup)
	propParams := db.ScanParams{
		FilterExpression: "EntityType = :entityType",
		ExpressionValues: map[string]interface{}{
			":entityType": "PROPERTY",
		},
	}
	propItems, err := s.db.Scan(ctx, propParams)
	if err != nil {
		return nil, fmt.Errorf("failed to scan properties: %w", err)
	}

	propMap := make(map[string]*properties.Property)
	for _, item := range propItems {
		var p properties.Property
		if err := attributevalue.UnmarshalMap(item, &p); err == nil {
			propMap[p.ID] = &p
		}
	}

	// 2. Fetch ALL agents via Scan
	userParams := db.ScanParams{
		FilterExpression: "EntityType = :entityType",
		ExpressionValues: map[string]interface{}{
			":entityType": "USER",
		},
	}
	userItems, err := s.db.Scan(ctx, userParams)
	if err != nil {
		return nil, fmt.Errorf("failed to scan users: %w", err)
	}

	userMap := make(map[string]string) // phone -> name
	for _, item := range userItems {
		var u users.User
		if err := attributevalue.UnmarshalMap(item, &u); err == nil {
			userMap[u.Phone] = u.Name
		}
	}

	// 3. Fetch ALL bookings via Scan (captures everything without date filtering issues)
	bookingParams := db.ScanParams{
		FilterExpression: "EntityType = :entityType",
		ExpressionValues: map[string]interface{}{
			":entityType": "BOOKING",
		},
	}
	bookingItems, err := s.db.Scan(ctx, bookingParams)
	if err != nil {
		return nil, fmt.Errorf("failed to scan bookings: %w", err)
	}

	var allBookings []bookings.Booking
	for _, item := range bookingItems {
		var b bookings.Booking
		if err := attributevalue.UnmarshalMap(item, &b); err == nil {
			allBookings = append(allBookings, b)
		}
	}

	// 4. Generate CSV
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
		// Resolve Agent Name from user map or fallback to stored name
		agentName := "Direct/Owner"
		if bk.BookedBy != "" {
			if name, ok := userMap[bk.BookedBy]; ok {
				agentName = name
			} else if bk.BookedByName != "" {
				agentName = bk.BookedByName
			} else {
				agentName = "Unknown Agent"
			}
		}

		// Resolve Property Metadata
		propertyName := bk.PropertyName
		ownerPhone := "Unknown"
		if p, ok := propMap[bk.PropertyID]; ok {
			propertyName = p.Name
			ownerPhone = p.OwnerID
		}

		row := []string{
			bk.ID, string(bk.Status), bk.CreatedAt.Format(time.RFC3339),
			propertyName, bk.PropertyID, ownerPhone,
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
