package analytics

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/properties"
)

// writeBookingsCSV writes the bookings section to the CSV writer, sorted by CreatedAt descending.
func writeBookingsCSV(w *csv.Writer, allBookings []bookings.Booking, propMap map[string]*properties.Property, userMap map[string]string) error {
	// Sort bookings by CreatedAt descending (latest first)
	sort.Slice(allBookings, func(i, j int) bool {
		return allBookings[i].CreatedAt.After(allBookings[j].CreatedAt)
	})

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
		return err
	}

	for _, bk := range allBookings {
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

		propertyName := bk.PropertyName
		ownerPhone := "Unknown"
		if p, ok := propMap[bk.PropertyID]; ok {
			propertyName = p.Name
			ownerPhone = p.OwnerID
		} else if strings.Contains(bk.PropertyID, "6c258855") {
			// Fallback for sample property
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
			return err
		}
	}

	return nil
}
