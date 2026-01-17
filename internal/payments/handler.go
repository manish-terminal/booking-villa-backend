package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
)

// Handler provides HTTP handlers for payment endpoints.
type Handler struct {
	service        *Service
	bookingService *bookings.Service
}

// NewHandler creates a new payment handler.
func NewHandler(dbClient *db.Client) *Handler {
	return &Handler{
		service:        NewService(dbClient),
		bookingService: bookings.NewService(dbClient),
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

// LogPaymentRequest represents a request to log an offline payment.
type LogPaymentRequest struct {
	Amount      float64       `json:"amount"`
	Method      PaymentMethod `json:"method"`
	Reference   string        `json:"reference,omitempty"`
	Notes       string        `json:"notes,omitempty"`
	PaymentDate string        `json:"paymentDate,omitempty"` // Format: 2006-01-02
}

// HandleLogPayment handles the POST /bookings/{id}/payments endpoint.
func (h *Handler) HandleLogPayment(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bookingID := request.PathParameters["id"]
	if bookingID == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var req LogPaymentRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// Validate amount
	if req.Amount <= 0 {
		return ErrorResponse(http.StatusBadRequest, "Amount must be greater than 0"), nil
	}

	// Validate payment method
	if !req.Method.IsValid() {
		return ErrorResponse(http.StatusBadRequest, "Invalid payment method. Valid values: cash, upi, bank_transfer, cheque, other"), nil
	}

	// Get booking to validate it exists
	booking, err := h.bookingService.GetBooking(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get booking"), nil
	}

	if booking == nil {
		return ErrorResponse(http.StatusNotFound, "Booking not found"), nil
	}

	// Parse payment date if provided
	paymentDate := time.Now()
	if req.PaymentDate != "" {
		parsed, err := time.Parse("2006-01-02", req.PaymentDate)
		if err != nil {
			return ErrorResponse(http.StatusBadRequest, "Invalid paymentDate format. Use YYYY-MM-DD"), nil
		}
		paymentDate = parsed
	}

	// Create payment record
	payment := &Payment{
		BookingID:   bookingID,
		Amount:      req.Amount,
		Currency:    booking.Currency,
		Method:      req.Method,
		Reference:   req.Reference,
		RecordedBy:  claims.Phone,
		Notes:       req.Notes,
		PaymentDate: paymentDate,
	}

	if err := h.service.LogPayment(ctx, payment); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to log payment"), nil
	}

	// Get updated payment summary
	summary, err := h.service.CalculatePaymentStatus(ctx, bookingID)
	if err != nil {
		// Payment was logged, but we couldn't get the summary
		return APIResponse(http.StatusCreated, map[string]interface{}{
			"payment": payment,
			"message": "Payment logged successfully",
		}), nil
	}

	return APIResponse(http.StatusCreated, map[string]interface{}{
		"payment": payment,
		"summary": summary,
		"message": "Payment logged successfully",
	}), nil
}

// HandleGetPayments handles the GET /bookings/{id}/payments endpoint.
func (h *Handler) HandleGetPayments(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bookingID := request.PathParameters["id"]
	if bookingID == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	// Get booking to validate it exists
	booking, err := h.bookingService.GetBooking(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get booking"), nil
	}

	if booking == nil {
		return ErrorResponse(http.StatusNotFound, "Booking not found"), nil
	}

	payments, err := h.service.GetPaymentsByBooking(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get payments"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	}), nil
}

// HandleGetPaymentStatus handles the GET /bookings/{id}/payment-status endpoint.
func (h *Handler) HandleGetPaymentStatus(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bookingID := request.PathParameters["id"]
	if bookingID == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	summary, err := h.service.CalculatePaymentStatus(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to calculate payment status: "+err.Error()), nil
	}

	return APIResponse(http.StatusOK, summary), nil
}

// PaymentHistoryResponse represents the full payment history for a booking.
type PaymentHistoryResponse struct {
	Booking  *bookings.Booking `json:"booking"`
	Summary  *PaymentSummary   `json:"summary"`
	Payments []*Payment        `json:"payments"`
}

// HandleGetPaymentHistory handles a combined endpoint to get booking with all payment info.
func (h *Handler) HandleGetPaymentHistory(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bookingID := request.PathParameters["id"]
	if bookingID == "" {
		return ErrorResponse(http.StatusBadRequest, "Booking ID is required"), nil
	}

	// Get booking
	booking, err := h.bookingService.GetBooking(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get booking"), nil
	}

	if booking == nil {
		return ErrorResponse(http.StatusNotFound, "Booking not found"), nil
	}

	// Get payments
	payments, err := h.service.GetPaymentsByBooking(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get payments"), nil
	}

	// Get summary
	summary, err := h.service.CalculatePaymentStatus(ctx, bookingID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to calculate payment status"), nil
	}

	return APIResponse(http.StatusOK, PaymentHistoryResponse{
		Booking:  booking,
		Summary:  summary,
		Payments: payments,
	}), nil
}
