package properties

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
)

// Handler provides HTTP handlers for property endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new property handler.
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

// CreatePropertyRequest represents a request to create a property.
type CreatePropertyRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Address       string   `json:"address"`
	City          string   `json:"city"`
	State         string   `json:"state,omitempty"`
	Country       string   `json:"country"`
	PricePerNight float64  `json:"pricePerNight"`
	Currency      string   `json:"currency,omitempty"`
	MaxGuests     int      `json:"maxGuests"`
	Bedrooms      int      `json:"bedrooms"`
	Bathrooms     int      `json:"bathrooms"`
	Amenities     []string `json:"amenities,omitempty"`
	Images        []string `json:"images,omitempty"`
}

// HandleCreateProperty handles the POST /properties endpoint.
func (h *Handler) HandleCreateProperty(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var req CreatePropertyRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// Validate required fields
	if req.Name == "" || req.Address == "" || req.City == "" || req.Country == "" {
		return ErrorResponse(http.StatusBadRequest, "Name, address, city, and country are required"), nil
	}

	property := &Property{
		Name:          req.Name,
		Description:   req.Description,
		Address:       req.Address,
		City:          req.City,
		State:         req.State,
		Country:       req.Country,
		OwnerID:       claims.Phone,
		PricePerNight: req.PricePerNight,
		Currency:      req.Currency,
		MaxGuests:     req.MaxGuests,
		Bedrooms:      req.Bedrooms,
		Bathrooms:     req.Bathrooms,
		Amenities:     req.Amenities,
		Images:        req.Images,
	}

	if err := h.service.CreateProperty(ctx, property); err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to create property"), nil
	}

	return APIResponse(http.StatusCreated, property), nil
}

// HandleGetProperty handles the GET /properties/{id} endpoint.
func (h *Handler) HandleGetProperty(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := request.PathParameters["id"]
	if id == "" {
		return ErrorResponse(http.StatusBadRequest, "Property ID is required"), nil
	}

	property, err := h.service.GetProperty(ctx, id)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get property"), nil
	}

	if property == nil {
		return ErrorResponse(http.StatusNotFound, "Property not found"), nil
	}

	return APIResponse(http.StatusOK, property), nil
}

// HandleListProperties handles the GET /properties endpoint.
func (h *Handler) HandleListProperties(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// For owners, list their properties
	// For admins, could list all (would need scan or different pattern)
	properties, err := h.service.ListPropertiesByOwner(ctx, claims.Phone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to list properties"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"properties": properties,
		"count":      len(properties),
	}), nil
}

// GenerateInviteCodeRequest represents a request to generate an invite code.
type GenerateInviteCodeRequest struct {
	ExpiresInDays int `json:"expiresInDays,omitempty"` // Default 30 days
	MaxUses       int `json:"maxUses,omitempty"`       // 0 = unlimited
}

// HandleGenerateInviteCode handles the POST /properties/{id}/invite-codes endpoint.
func (h *Handler) HandleGenerateInviteCode(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	propertyID := request.PathParameters["id"]
	if propertyID == "" {
		return ErrorResponse(http.StatusBadRequest, "Property ID is required"), nil
	}

	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Check if user owns the property (or is admin)
	property, err := h.service.GetProperty(ctx, propertyID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get property"), nil
	}

	if property == nil {
		return ErrorResponse(http.StatusNotFound, "Property not found"), nil
	}

	if property.OwnerID != claims.Phone && claims.Role != "admin" {
		return ErrorResponse(http.StatusForbidden, "You don't own this property"), nil
	}

	var req GenerateInviteCodeRequest
	if request.Body != "" {
		if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
			return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
		}
	}

	// Default expiry: 30 days
	expiresInDays := req.ExpiresInDays
	if expiresInDays <= 0 {
		expiresInDays = 30
	}
	expiresAt := time.Now().AddDate(0, 0, expiresInDays)

	inviteCode, err := h.service.GenerateInviteCode(ctx, propertyID, claims.Phone, expiresAt, req.MaxUses)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to generate invite code: "+err.Error()), nil
	}

	return APIResponse(http.StatusCreated, inviteCode), nil
}

// ValidateInviteCodeRequest represents a request to validate an invite code.
type ValidateInviteCodeRequest struct {
	Code string `json:"code"`
}

// HandleValidateInviteCode handles the POST /invite-codes/validate endpoint.
func (h *Handler) HandleValidateInviteCode(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req ValidateInviteCodeRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	if req.Code == "" {
		return ErrorResponse(http.StatusBadRequest, "Invite code is required"), nil
	}

	inviteCode, err := h.service.ValidateInviteCode(ctx, req.Code)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, err.Error()), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"valid":      true,
		"inviteCode": inviteCode,
		"message":    "Invite code is valid",
	}), nil
}

// HandleListInviteCodes handles the GET /properties/{id}/invite-codes endpoint.
func (h *Handler) HandleListInviteCodes(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	propertyID := request.PathParameters["id"]
	if propertyID == "" {
		return ErrorResponse(http.StatusBadRequest, "Property ID is required"), nil
	}

	// Get user from context
	claims, ok := middleware.GetClaimsFromContext(ctx)
	if !ok {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// Check if user owns the property
	property, err := h.service.GetProperty(ctx, propertyID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to get property"), nil
	}

	if property == nil {
		return ErrorResponse(http.StatusNotFound, "Property not found"), nil
	}

	if property.OwnerID != claims.Phone && claims.Role != "admin" {
		return ErrorResponse(http.StatusForbidden, "You don't own this property"), nil
	}

	codes, err := h.service.ListInviteCodesByProperty(ctx, propertyID)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, "Failed to list invite codes"), nil
	}

	return APIResponse(http.StatusOK, map[string]interface{}{
		"inviteCodes": codes,
		"count":       len(codes),
	}), nil
}
