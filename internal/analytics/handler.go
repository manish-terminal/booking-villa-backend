package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
)

// Handler provides HTTP handlers for analytics endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new analytics handler.
func NewHandler(dbClient *db.Client) *Handler {
	return &Handler{
		service: NewService(dbClient),
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

// HandleOwnerAnalytics handles GET /analytics/owner endpoint.
func (h *Handler) HandleOwnerAnalytics(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Parse date range from query params
	startDate, endDate := parseDateRange(request)

	analytics, err := h.service.GetOwnerAnalytics(ctx, claims.Phone, startDate, endDate)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get analytics: "+err.Error()), nil
	}

	return APIResponse(http.StatusOK, analytics), nil
}

// HandleAgentAnalytics handles GET /analytics/agent endpoint.
func (h *Handler) HandleAgentAnalytics(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Parse date range from query params
	startDate, endDate := parseDateRange(request)

	analytics, err := h.service.GetAgentAnalytics(ctx, claims.Phone, startDate, endDate)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get analytics: "+err.Error()), nil
	}

	return APIResponse(http.StatusOK, analytics), nil
}

// HandleAgentPropertyPerformance handles GET /analytics/agent/property-performance endpoint.
func (h *Handler) HandleAgentPropertyPerformance(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Parse date range from query params
	startDate, endDate := parseDateRange(request)

	performance, err := h.service.GetAgentPropertyPerformance(ctx, claims.Phone, startDate, endDate)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get property performance: "+err.Error()), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"data": performance,
	}), nil
}

// HandleDashboard handles GET /analytics/dashboard endpoint.
func (h *Handler) HandleDashboard(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	stats, err := h.service.GetDashboardStats(ctx, claims.Phone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get dashboard stats: "+err.Error()), nil
	}

	return APIResponse(http.StatusOK, stats), nil
}

// parseDateRange extracts start and end dates from query params.
// Defaults to current month if not provided.
func parseDateRange(request events.APIGatewayProxyRequest) (time.Time, time.Time) {
	now := time.Now()

	// Default: current month
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	// Parse from query params if provided
	if startStr := request.QueryStringParameters["startDate"]; startStr != "" {
		if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			startDate = parsed
		}
	}

	if endStr := request.QueryStringParameters["endDate"]; endStr != "" {
		if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			endDate = parsed.Add(24*time.Hour - time.Second)
		}
	}

	return startDate, endDate
}
