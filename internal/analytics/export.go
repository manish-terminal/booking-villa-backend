package analytics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// GenerateMasterCSV creates a CSV dump of all data.
func (s *Service) GenerateMasterCSV(ctx context.Context) ([]byte, error) {
	// 1. Fetch ALL properties
	propParams := db.ScanParams{
		FilterExpression: "begins_with(PK, :prefix) AND SK = :sk",
		ExpressionValues: map[string]interface{}{
			":prefix": "PROPERTY#",
			":sk":     "METADATA",
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

	// 2. Fetch ALL users
	userParams := db.ScanParams{
		FilterExpression: "begins_with(PK, :prefix) AND SK = :sk",
		ExpressionValues: map[string]interface{}{
			":prefix": "USER#",
			":sk":     "PROFILE",
		},
	}
	userItems, err := s.db.Scan(ctx, userParams)
	if err != nil {
		return nil, fmt.Errorf("failed to scan users: %w", err)
	}

	var allUsers []users.User
	userMap := make(map[string]string) // phone -> name
	for _, item := range userItems {
		var u users.User
		if err := attributevalue.UnmarshalMap(item, &u); err == nil {
			userMap[u.Phone] = u.Name
			allUsers = append(allUsers, u)
		}
	}

	// 3. Fetch ALL bookings
	bookingParams := db.ScanParams{
		FilterExpression: "begins_with(PK, :prefix) AND SK = :sk",
		ExpressionValues: map[string]interface{}{
			":prefix": "BOOKING#",
			":sk":     "METADATA",
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

	// 4. Generate CSV with all sections
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Section 1: Bookings (latest first)
	if err := w.Write([]string{"=== BOOKINGS (Latest First) ==="}); err != nil {
		return nil, err
	}
	if err := writeBookingsCSV(w, allBookings, propMap, userMap); err != nil {
		return nil, err
	}

	// Section 2: Users with Roles (recent first)
	if err := w.Write([]string{}); err != nil {
		return nil, err
	}
	if err := w.Write([]string{"=== USERS WITH ROLES (Recent First) ==="}); err != nil {
		return nil, err
	}
	if err := writeUsersCSV(w, allUsers); err != nil {
		return nil, err
	}

	// Section 3: Agents with Connections (recent first)
	if err := w.Write([]string{}); err != nil {
		return nil, err
	}
	if err := w.Write([]string{"=== AGENTS WITH CONNECTIONS (Recent First) ==="}); err != nil {
		return nil, err
	}
	if err := writeAgentsCSV(w, allUsers, propMap); err != nil {
		return nil, err
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}
