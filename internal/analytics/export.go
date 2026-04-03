package analytics

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
	"github.com/xuri/excelize/v2"
)

// GenerateBookingsExcel creates an Excel file with booking data.
func (s *Service) GenerateBookingsExcel(ctx context.Context) ([]byte, error) {
	propMap, err := s.scanProperties(ctx)
	if err != nil {
		return nil, err
	}
	allUsers, err := s.scanAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	userMap := make(map[string]string)
	for _, u := range allUsers {
		userMap[u.Phone] = u.Name
	}
	allBookings, err := s.scanAllBookings(ctx)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()
	f.NewSheet("Bookings")
	if err := writeBookingsSheet(f, allBookings, propMap, userMap); err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateUsersExcel creates an Excel file with user data.
func (s *Service) GenerateUsersExcel(ctx context.Context) ([]byte, error) {
	allUsers, err := s.scanAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()
	f.NewSheet("Users")
	if err := writeUsersSheet(f, allUsers); err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateAgentsExcel creates an Excel file with agent data.
func (s *Service) GenerateAgentsExcel(ctx context.Context) ([]byte, error) {
	propMap, err := s.scanProperties(ctx)
	if err != nil {
		return nil, err
	}
	allUsers, err := s.scanAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()
	f.NewSheet("Agents")
	if err := writeAgentsSheet(f, allUsers, propMap); err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Internal writing helpers (Consolidated from previous implementation)

func writeBookingsSheet(f *excelize.File, bks []bookings.Booking, propMap map[string]*properties.Property, userMap map[string]string) error {
	sheet := "Bookings"
	headers := []string{"Booking ID", "Status", "Created At", "Property Name", "Property ID", "Owner Phone", "Guest Name", "Guest Phone", "Guest Email", "Num Guests", "Check In", "Check Out", "Nights", "Total Amount", "Agent Commission", "Currency", "Booked By Phone", "Booked By Name", "Invite Code", "Notes"}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	sort.Slice(bks, func(i, j int) bool { return bks[i].CreatedAt.After(bks[j].CreatedAt) })
	for row, bk := range bks {
		agentName := "Direct/Owner"
		if bk.BookedBy != "" {
			if name, ok := userMap[bk.BookedBy]; ok { agentName = name } else if bk.BookedByName != "" { agentName = bk.BookedByName }
		}
		propertyName := bk.PropertyName
		ownerPhone := "Unknown"
		if p, ok := propMap[bk.PropertyID]; ok { propertyName = p.Name; ownerPhone = p.OwnerID }
		values := []interface{}{bk.ID, string(bk.Status), bk.CreatedAt.Format(time.RFC3339), propertyName, bk.PropertyID, ownerPhone, bk.GuestName, bk.GuestPhone, bk.GuestEmail, bk.NumGuests, bk.CheckIn.Format("2006-01-02"), bk.CheckOut.Format("2006-01-02"), bk.NumNights, bk.TotalAmount, bk.AgentCommission, bk.Currency, bk.BookedBy, agentName, bk.InviteCode, bk.Notes}
		for col, val := range values { cell, _ := excelize.CoordinatesToCellName(col+1, row+2); f.SetCellValue(sheet, cell, val) }
	}
	for i := range headers { colName, _ := excelize.ColumnNumberToName(i + 1); f.SetColWidth(sheet, colName, colName, 18) }
	return nil
}

func writeUsersSheet(f *excelize.File, usersArr []users.User) error {
	sheet := "Users"
	headers := []string{"Name", "Phone", "Email", "Role", "Status", "Created At"}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"548235"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	sort.Slice(usersArr, func(i, j int) bool { return usersArr[i].CreatedAt.After(usersArr[j].CreatedAt) })
	for row, u := range usersArr {
		values := []interface{}{u.Name, u.Phone, u.Email, string(u.Role), string(u.Status), u.CreatedAt.Format(time.RFC3339)}
		for col, val := range values { cell, _ := excelize.CoordinatesToCellName(col+1, row+2); f.SetCellValue(sheet, cell, val) }
	}
	for i := range headers { colName, _ := excelize.ColumnNumberToName(i + 1); f.SetColWidth(sheet, colName, colName, 22) }
	return nil
}

func writeAgentsSheet(f *excelize.File, usersArr []users.User, propMap map[string]*properties.Property) error {
	sheet := "Agents"
	headers := []string{"Agent Name", "Phone", "Email", "Status", "Managed Properties", "Created At"}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"BF8F00"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	var agents []users.User
	for _, u := range usersArr { if u.Role == "agent" { agents = append(agents, u) } }
	sort.Slice(agents, func(i, j int) bool { return agents[i].CreatedAt.After(agents[j].CreatedAt) })
	for row, agent := range agents {
		var propNames []string
		for _, pid := range agent.ManagedProperties {
			if p, ok := propMap[pid]; ok { propNames = append(propNames, p.Name) } else { propNames = append(propNames, pid) }
		}
		values := []interface{}{agent.Name, agent.Phone, agent.Email, string(agent.Status), strings.Join(propNames, "; "), agent.CreatedAt.Format(time.RFC3339)}
		for col, val := range values { cell, _ := excelize.CoordinatesToCellName(col+1, row+2); f.SetCellValue(sheet, cell, val) }
	}
	for i := range headers { colName, _ := excelize.ColumnNumberToName(i + 1); f.SetColWidth(sheet, colName, colName, 25) }
	return nil
}

// Data fetching helpers

func (s *Service) scanProperties(ctx context.Context) (map[string]*properties.Property, error) {
	items, err := s.db.Scan(ctx, db.ScanParams{FilterExpression: "begins_with(PK, :prefix) AND SK = :sk", ExpressionValues: map[string]interface{}{":prefix": "PROPERTY#", ":sk": "METADATA"}})
	if err != nil { return nil, err }
	propMap := make(map[string]*properties.Property)
	for _, item := range items {
		var p properties.Property
		if err := attributevalue.UnmarshalMap(item, &p); err == nil { propMap[p.ID] = &p }
	}
	return propMap, nil
}

func (s *Service) scanAllUsers(ctx context.Context) ([]users.User, error) {
	items, err := s.db.Scan(ctx, db.ScanParams{FilterExpression: "begins_with(PK, :prefix) AND SK = :sk", ExpressionValues: map[string]interface{}{":prefix": "USER#", ":sk": "PROFILE"}})
	if err != nil { return nil, err }
	var allUsers []users.User
	for _, item := range items {
		var u users.User
		if err := attributevalue.UnmarshalMap(item, &u); err == nil { allUsers = append(allUsers, u) }
	}
	return allUsers, nil
}

func (s *Service) scanAllBookings(ctx context.Context) ([]bookings.Booking, error) {
	items, err := s.db.Scan(ctx, db.ScanParams{FilterExpression: "begins_with(PK, :prefix) AND SK = :sk", ExpressionValues: map[string]interface{}{":prefix": "BOOKING#", ":sk": "METADATA"}})
	if err != nil { return nil, err }
	var bks []bookings.Booking
	for _, item := range items {
		var b bookings.Booking
		if err := attributevalue.UnmarshalMap(item, &b); err == nil { bks = append(bks, b) }
	}
	return bks, nil
}
