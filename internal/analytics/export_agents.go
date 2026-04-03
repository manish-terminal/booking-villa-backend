package analytics

import (
	"encoding/csv"
	"strings"
	"time"

	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// writeAgentsCSV writes the agents section to the CSV writer, sorted by CreatedAt descending.
// It filters agents from the allUsers slice (already sorted by CreatedAt desc) and resolves property names.
func writeAgentsCSV(w *csv.Writer, allUsers []users.User, propMap map[string]*properties.Property) error {
	// Filter only agents (allUsers is already sorted by CreatedAt desc)
	var agents []users.User
	for _, u := range allUsers {
		if u.Role == users.RoleAgent {
			agents = append(agents, u)
		}
	}

	header := []string{
		"Agent Name", "Phone", "Email", "Status",
		"Managed Properties (IDs)", "Managed Properties (Names)",
		"Created At",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, agent := range agents {
		propIDs := strings.Join(agent.ManagedProperties, "; ")
		var propNames []string
		for _, pid := range agent.ManagedProperties {
			if p, ok := propMap[pid]; ok {
				propNames = append(propNames, p.Name)
			} else {
				propNames = append(propNames, pid)
			}
		}

		row := []string{
			agent.Name, agent.Phone, agent.Email, string(agent.Status),
			propIDs, strings.Join(propNames, "; "),
			agent.CreatedAt.Format(time.RFC3339),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
