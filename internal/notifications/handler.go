// Package notifications provides in-app notification services.
package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
)

// Handler provides HTTP handlers for notification endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new notification handler.
func NewHandler(dbClient *db.Client) *Handler {
	return &Handler{
		service: NewService(dbClient),
	}
}

// GetService returns the notification service (for use in other handlers).
func (h *Handler) GetService() *Service {
	return h.service
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

// HandleListNotifications handles the GET /notifications endpoint.
func (h *Handler) HandleListNotifications(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Parse query parameters
	limitStr := request.QueryStringParameters["limit"]
	unreadOnlyStr := request.QueryStringParameters["unreadOnly"]

	limit := int32(50) // Default limit
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	unreadOnly := false
	if unreadOnlyStr == "true" {
		unreadOnly = true
	}

	notifications, err := h.service.GetNotificationsByUser(ctx, claims.Phone, limit, unreadOnly)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get notifications"), nil
	}

	// Convert to response format
	responses := make([]NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		responses = append(responses, n.ToResponse())
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"notifications": responses,
		"count":         len(responses),
	}), nil
}

// HandleMarkAsRead handles the PATCH /notifications/{id}/read endpoint.
func (h *Handler) HandleMarkAsRead(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	notificationID := request.PathParameters["id"]
	if notificationID == "" {
		return ErrorResponse(http.StatusBadRequest, "Notification ID is required"), nil
	}

	if err := h.service.MarkAsRead(ctx, notificationID, claims.Phone); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to mark notification as read"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"message":        "Notification marked as read",
		"notificationId": notificationID,
	}), nil
}

// HandleMarkAllAsRead handles the POST /notifications/mark-all-read endpoint.
func (h *Handler) HandleMarkAllAsRead(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	count, err := h.service.MarkAllAsRead(ctx, claims.Phone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to mark notifications as read"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"message": "All notifications marked as read",
		"count":   count,
	}), nil
}

// HandleGetUnreadCount handles the GET /notifications/count endpoint.
func (h *Handler) HandleGetUnreadCount(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	count, err := h.service.GetUnreadCount(ctx, claims.Phone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get unread count"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"unreadCount": count,
	}), nil
}
