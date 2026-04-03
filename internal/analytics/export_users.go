package analytics

import (
	"encoding/csv"
	"sort"
	"time"

	"github.com/booking-villa-backend/internal/users"
)

// writeUsersCSV writes the users section to the CSV writer, sorted by CreatedAt descending.
func writeUsersCSV(w *csv.Writer, allUsers []users.User) error {
	// Sort users by CreatedAt descending (recent first)
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].CreatedAt.After(allUsers[j].CreatedAt)
	})

	header := []string{
		"Name", "Phone", "Email", "Role", "Status", "Created At", "Updated At",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, u := range allUsers {
		row := []string{
			u.Name, u.Phone, u.Email, string(u.Role), string(u.Status),
			u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
