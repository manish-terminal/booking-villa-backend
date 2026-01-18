package bookings

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// Handler provides HTTP handlers for booking endpoints.
type Handler struct {
	service         *Service
	propertyService *properties.Service
	userService     *users.Service
}

// NewHandler creates a new booking handler.
func NewHandler(dbClient *db.Client) *Handler {
	return &Handler{
		service:         NewService(dbClient),
		propertyService: properties.NewService(dbClient),
		userService:     users.NewService(dbClient),
	}
}

// APIResponse creates a standardized API Gateway response.
func APIResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
	jsonBody, _ := json.Marshal(body)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,Authorization",
		},
		Body: string(jsonBody),
	}
}

// ErrorResponse creates a standardized error response.
func ErrorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return APIResponse(statusCode, map[string]string{"error": message})
}

// CreateBookingRequest represents a request to create a booking.
type CreateBookingRequest struct {
	PropertyID      string  `json:"propertyId"`
	GuestName       string  `json:"guestName"`
	GuestPhone      string  `json:"guestPhone"`
	GuestEmail      string  `json:"guestEmail,omitempty"`
	NumGuests       int     `json:"numGuests"`
	CheckIn         string  `json:"checkIn"`  // Format: 2006-01-02
	CheckOut        string  `json:"checkOut"` // Format: 2006-01-02
	Notes           string  `json:"notes,omitempty"`
	SpecialRequests string  `json:"specialRequests,omitempty"`
	InviteCode      string  `json:"inviteCode,omitempty"`
	PricePerNight   float64 `json:"pricePerNight,omitempty"` // Override property price if needed
	TotalAmount     float64 `json:"totalAmount,omitempty"`   // Directly set total amount for dynamic pricing
}

// HandleCreateBooking handles the POST /bookings endpoint.
func (h *Handler) HandleCreateBooking(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var req CreateBookingRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// Validate required fields
	if req.PropertyID == "" || req.GuestName == "" || req.GuestPhone == "" ||
		req.CheckIn == "" || req.CheckOut == "" {
		return ErrorResponse(http.StatusBadRequest, "PropertyID, guestName, guestPhone, checkIn, and checkOut are required"), nil
	}

	// Parse dates
	checkIn, err := time.Parse("2006-01-02", req.CheckIn)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid checkIn date format. Use YYYY-MM-DD"), nil
	}

	checkOut, err := time.Parse("2006-01-02", req.CheckOut)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid checkOut date format. Use YYYY-MM-DD"), nil
	}

	// Validate dates
	if checkIn.After(checkOut) || checkIn.Equal(checkOut) {
		return ErrorResponse(http.StatusBadRequest, "Check-out must be after check-in"), nil
	}

	if checkIn.Before(time.Now().Truncate(24 * time.Hour)) {
		return ErrorResponse(http.StatusBadRequest, "Check-in cannot be in the past"), nil
	}

	// Get property to validate and get pricing
	property, err := h.propertyService.GetProperty(ctx, req.PropertyID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get property"), nil
	}

	if property == nil {
		return ErrorResponse(http.StatusNotFound, "Property not found"), nil
	}

	if !property.IsActive {
		return ErrorResponse(http.StatusBadRequest, "Property is not active"), nil
	}

	// Check availability
	available, err := h.service.CheckAvailability(ctx, req.PropertyID, checkIn, checkOut)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to check availability"), nil
	}

	if !available {
		return ErrorResponse(http.StatusConflict, "Property is not available for the selected dates"), nil
	}

	// Validate invite code if provided (for agents)
	if req.InviteCode != "" {
		inviteCode, err := h.propertyService.ValidateInviteCode(ctx, req.InviteCode)
		if err != nil {
			return ErrorResponse(http.StatusBadRequest, "Invalid invite code: "+err.Error()), nil
		}

		if inviteCode.PropertyID != req.PropertyID {
			return ErrorResponse(http.StatusBadRequest, "Invite code is for a different property"), nil
		}

		// Mark invite code as used
		_ = h.propertyService.UseInviteCode(ctx, req.InviteCode, req.PropertyID)
	}

	// Use property price unless overridden
	pricePerNight := property.PricePerNight
	if req.PricePerNight > 0 {
		pricePerNight = req.PricePerNight
	}

	// Set default number of guests
	numGuests := req.NumGuests
	if numGuests <= 0 {
		numGuests = 1
	}

	// Create booking
	booking := &Booking{
		PropertyID:      req.PropertyID,
		PropertyName:    property.Name,
		GuestName:       req.GuestName,
		GuestPhone:      req.GuestPhone,
		GuestEmail:      req.GuestEmail,
		NumGuests:       numGuests,
		CheckIn:         checkIn,
		CheckOut:        checkOut,
		PricePerNight:   pricePerNight,
		TotalAmount:     req.TotalAmount, // Support dynamic total price
		Currency:        property.Currency,
		BookedBy:        claims.Phone,
		InviteCode:      req.InviteCode,
		Notes:           req.Notes,
		SpecialRequests: req.SpecialRequests,
		Status:          StatusPendingConfirmation,
	}

	if err := h.service.CreateBooking(ctx, booking); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to create booking"), nil
	}

	return APIResponse(http.StatusCreated, booking), nil
}

// HandleGetBooking handles the GET /bookings/{id} endpoint.
func (h *Handler) HandleGetBooking(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := request.PathParameters["id"]
	if id == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	booking, err := h.service.GetBooking(ctx, id)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get booking"), nil
	}

	if booking == nil {
		return ErrorResponse(http.StatusNotFound, "Booking not found"), nil
	}

	// Note: In production, add permission check here
	// to ensure user can view this booking

	return APIResponse(http.StatusOK, booking), nil
}

// HandleListBookings handles the GET /bookings endpoint.
func (h *Handler) HandleListBookings(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get query parameters
	propertyID := request.QueryStringParameters["propertyId"]
	startDate := request.QueryStringParameters["startDate"]
	endDate := request.QueryStringParameters["endDate"]

	if propertyID == "" {
		return ErrorResponse(http.StatusBadRequest, "PropertyId query parameter is required"), nil
	}

	var dateRange *DateRange
	if startDate != "" && endDate != "" {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return ErrorResponse(http.StatusBadRequest, "Invalid startDate format"), nil
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return ErrorResponse(http.StatusBadRequest, "Invalid endDate format"), nil
		}
		dateRange = &DateRange{Start: start, End: end}
	}

	bookings, err := h.service.ListBookingsByProperty(ctx, propertyID, dateRange)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to list bookings"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"bookings": bookings,
		"count":    len(bookings),
	}), nil
}

// UpdateBookingStatusRequest represents a request to update booking status.
type UpdateBookingStatusRequest struct {
	Status BookingStatus `json:"status"`
}

// HandleUpdateBookingStatus handles the PATCH /bookings/{id}/status endpoint.
func (h *Handler) HandleUpdateBookingStatus(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := request.PathParameters["id"]
	if id == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	var req UpdateBookingStatusRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	if !req.Status.IsValid() {
		return ErrorResponse(http.StatusBadRequest, "Invalid status. Valid values: pending_confirmation, confirmed, checked_in, checked_out, cancelled, no_show"), nil
	}

	// Get booking to verify it exists
	booking, err := h.service.GetBooking(ctx, id)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get booking"), nil
	}

	if booking == nil {
		return ErrorResponse(http.StatusNotFound, "Booking not found"), nil
	}

	// Permission check
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	if claims.Role != string(users.RoleAdmin) && claims.Role != string(users.RoleOwner) {
		// If agent, check if they are authorized for the property
		authorized, err := h.userService.IsAuthorizedForProperty(ctx, claims.Phone, booking.PropertyID)
		if err != nil {
			return ErrorResponse(http.StatusInternalServerError, "Authorization check failed"), nil
		}
		if !authorized && booking.BookedBy != claims.Phone {
			return ErrorResponse(http.StatusForbidden, "Insufficient permissions to update this booking"), nil
		}
	}

	// Update status
	if err := h.service.UpdateBookingStatus(ctx, id, req.Status); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to update booking status"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"message":   "Booking status updated",
		"bookingId": id,
		"status":    req.Status,
	}), nil
}

// HandleCheckAvailability handles the GET /properties/{id}/availability endpoint.
func (h *Handler) HandleCheckAvailability(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	propertyID := request.PathParameters["id"]
	if propertyID == "" {
		return ErrorResponse(http.StatusBadRequest, "Property ID is required"), nil
	}

	checkInStr := request.QueryStringParameters["checkIn"]
	checkOutStr := request.QueryStringParameters["checkOut"]

	if checkInStr == "" || checkOutStr == "" {
		return ErrorResponse(http.StatusBadRequest, "checkIn and checkOut query parameters are required"), nil
	}

	checkIn, err := time.Parse("2006-01-02", checkInStr)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid checkIn date format. Use YYYY-MM-DD"), nil
	}

	checkOut, err := time.Parse("2006-01-02", checkOutStr)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid checkOut date format. Use YYYY-MM-DD"), nil
	}

	available, err := h.service.CheckAvailability(ctx, propertyID, checkIn, checkOut)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to check availability"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"propertyId": propertyID,
		"checkIn":    checkInStr,
		"checkOut":   checkOutStr,
		"available":  available,
	}), nil
}

// OccupiedDateRange represents a range of dates that are not available.
type OccupiedDateRange struct {
	CheckIn  time.Time `json:"checkIn"`
	CheckOut time.Time `json:"checkOut"`
	Status   string    `json:"status"`
}

// HandleGetPropertyCalendar handles the GET /properties/{id}/calendar endpoint.
func (h *Handler) HandleGetPropertyCalendar(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	propertyID := request.PathParameters["id"]
	if propertyID == "" {
		return ErrorResponse(http.StatusBadRequest, "Property ID is required"), nil
	}

	startDateStr := request.QueryStringParameters["startDate"]
	endDateStr := request.QueryStringParameters["endDate"]

	// Default to current month if not provided
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0) // End of current month

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = t
		}
	}

	bookings, err := h.service.ListBookingsByProperty(ctx, propertyID, &DateRange{
		Start: startDate,
		End:   endDate,
	})
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get bookings: "+err.Error()), nil
	}

	occupied := make([]OccupiedDateRange, 0)
	for _, b := range bookings {
		// Only include non-cancelled and non-no-show bookings as occupied
		if b.Status != StatusCancelled && b.Status != StatusNoShow {
			occupied = append(occupied, OccupiedDateRange{
				CheckIn:  b.CheckIn,
				CheckOut: b.CheckOut,
				Status:   string(b.Status),
			})
		}
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"propertyId": propertyID,
		"startDate":  startDate.Format("2006-01-02"),
		"endDate":    endDate.Format("2006-01-02"),
		"occupied":   occupied,
	}), nil
}
