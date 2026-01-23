package payments

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/users"
)

// Handler provides HTTP handlers for payment endpoints.
type Handler struct {
	service        *Service
	bookingService *bookings.Service
	userService    *users.Service
}

// NewHandler creates a new payment handler.
func NewHandler(dbClient *db.Client) *Handler {
	return &Handler{
		service:        NewService(dbClient),
		bookingService: bookings.NewService(dbClient),
		userService:    users.NewService(dbClient),
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
