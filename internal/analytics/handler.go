package analytics

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/middleware"
)

// Handler provides HTTP handlers for analytics endpoints.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleExportBookings handles the bookings export request.
func (h *Handler) HandleExportBookings(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	data, err := h.service.GenerateBookingsExcel(ctx)
	if err != nil {
		return h.errorResponse(500, "Failed to generate bookings export")
	}
	return h.excelResponse(data, "bookings_export")
}

// HandleExportUsers handles the users export request.
func (h *Handler) HandleExportUsers(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	data, err := h.service.GenerateUsersExcel(ctx)
	if err != nil {
		return h.errorResponse(500, "Failed to generate users export")
	}
	return h.excelResponse(data, "users_export")
}

// HandleExportAgents handles the agents export request.
func (h *Handler) HandleExportAgents(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	data, err := h.service.GenerateAgentsExcel(ctx)
	if err != nil {
		return h.errorResponse(500, "Failed to generate agents export")
	}
	return h.excelResponse(data, "agents_export")
}

// excelResponse is a helper to return a base64-encoded Excel file.
func (h *Handler) excelResponse(data []byte, prefix string) (events.APIGatewayProxyResponse, error) {
	filename := fmt.Sprintf("%s_%s.xlsx", prefix, time.Now().Format("2006-01-02"))
	return events.APIGatewayProxyResponse{
		StatusCode:      200,
		IsBase64Encoded: true,
		Headers: map[string]string{
			"Content-Type":                 "application/octet-stream",
			"Content-Disposition":          fmt.Sprintf("attachment; filename=\"%s\"", filename),
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,Authorization",
		},
		Body: base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (h *Handler) HandleOwnerAnalytics(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ownerPhone := request.QueryStringParameters["owner_phone"]
	if ownerPhone == "" {
		claims, ok := middleware.GetClaimsFromContext(ctx)
		if !ok {
			return h.errorResponse(401, "Unauthorized")
		}
		ownerPhone = claims.Phone
	}
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	data, err := h.service.GetOwnerAnalytics(ctx, ownerPhone, startDate, endDate)
	if err != nil {
		return h.errorResponse(500, "Failed to get analytics")
	}
	return h.apiResponse(200, data)
}

func (h *Handler) HandleAgentAnalytics(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	agentPhone := request.QueryStringParameters["agent_phone"]
	if agentPhone == "" {
		claims, ok := middleware.GetClaimsFromContext(ctx)
		if !ok {
			return h.errorResponse(401, "Unauthorized")
		}
		agentPhone = claims.Phone
	}
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	data, err := h.service.GetAgentAnalytics(ctx, agentPhone, startDate, endDate)
	if err != nil {
		return h.errorResponse(500, "Failed to get agent analytics")
	}
	return h.apiResponse(200, data)
}

func (h *Handler) HandleAgentPropertyPerformance(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	agentPhone := request.QueryStringParameters["agent_phone"]
	if agentPhone == "" {
		claims, ok := middleware.GetClaimsFromContext(ctx)
		if !ok {
			return h.errorResponse(401, "Unauthorized")
		}
		agentPhone = claims.Phone
	}
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	data, err := h.service.GetAgentPropertyPerformance(ctx, agentPhone, startDate, endDate)
	if err != nil {
		return h.errorResponse(500, "Failed to get performance data")
	}
	return h.apiResponse(200, data)
}

func (h *Handler) HandleDashboard(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return h.errorResponse(401, "Unauthorized")
	}
	data, err := h.service.GetDashboardStats(ctx, claims.Phone)
	if err != nil {
		return h.errorResponse(500, "Failed to get dashboard data")
	}
	return h.apiResponse(200, data)
}

func (h *Handler) apiResponse(status int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, _ := json.Marshal(body)
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,Authorization",
		},
		Body: string(jsonBody),
	}, nil
}

func (h *Handler) errorResponse(status int, message string) (events.APIGatewayProxyResponse, error) {
	return h.apiResponse(status, map[string]string{"error": message})
}
