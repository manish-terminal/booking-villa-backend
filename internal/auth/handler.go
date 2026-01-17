package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/utils"
)

// Handler provides HTTP handlers for authentication endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new auth handler.
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

// HandleSendOTP handles the POST /auth/send-otp endpoint.
func (h *Handler) HandleSendOTP(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req SendOTPRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	if req.Phone == "" {
		return ErrorResponse(http.StatusBadRequest, "Phone number is required"), nil
	}

	code, err := h.service.SendOTP(ctx, req.Phone)
	if err != nil {
		return ErrorResponse(http.StatusInternalServerError, err.Error()), nil
	}

	// In production, don't return the code in response
	// This is included for testing/development purposes
	return APIResponse(http.StatusOK, map[string]interface{}{
		"message": "OTP sent successfully",
		"phone":   req.Phone,
		// Remove this in production:
		"code": code,
	}), nil
}

// HandleVerifyOTP handles the POST /auth/verify-otp endpoint.
func (h *Handler) HandleVerifyOTP(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req VerifyOTPRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	result, err := h.service.VerifyOTP(ctx, req)
	if err != nil {
		return ErrorResponse(http.StatusUnauthorized, err.Error()), nil
	}

	// If no token, user is pending approval
	if result.Token == "" {
		return APIResponse(http.StatusAccepted, result), nil
	}

	return APIResponse(http.StatusOK, result), nil
}

// HandleLogin handles the POST /auth/login endpoint.
func (h *Handler) HandleLogin(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req LoginRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	result, err := h.service.LoginWithPassword(ctx, req)
	if err != nil {
		return ErrorResponse(http.StatusUnauthorized, err.Error()), nil
	}

	return APIResponse(http.StatusOK, result), nil
}

// HandleRefreshToken handles the POST /auth/refresh endpoint.
func (h *Handler) HandleRefreshToken(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Extract token from Authorization header
	authHeader := request.Headers["Authorization"]
	if authHeader == "" {
		authHeader = request.Headers["authorization"]
	}

	tokenString, err := utils.ExtractTokenFromHeader(authHeader)
	if err != nil {
		return ErrorResponse(http.StatusUnauthorized, "Invalid authorization header"), nil
	}

	result, err := h.service.RefreshToken(ctx, tokenString)
	if err != nil {
		return ErrorResponse(http.StatusUnauthorized, err.Error()), nil
	}

	return APIResponse(http.StatusOK, result), nil
}

// HandleSetPassword handles the POST /users/password endpoint.
func (h *Handler) HandleSetPassword(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user from context (set by auth middleware)
	claims, err := extractClaimsFromRequest(request)
	if err != nil {
		return ErrorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	var req SetPasswordRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return ErrorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// Use the authenticated user's phone
	phone := claims.Phone

	err = h.service.SetPassword(ctx, phone, req.Password, req.OldPassword)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, err.Error()), nil
	}

	return APIResponse(http.StatusOK, map[string]string{
		"message": "Password set successfully",
	}), nil
}

// extractClaimsFromRequest extracts JWT claims from the request.
func extractClaimsFromRequest(request events.APIGatewayProxyRequest) (*utils.TokenClaims, error) {
	authHeader := request.Headers["Authorization"]
	if authHeader == "" {
		authHeader = request.Headers["authorization"]
	}

	tokenString, err := utils.ExtractTokenFromHeader(authHeader)
	if err != nil {
		return nil, err
	}

	return utils.ValidateToken(tokenString)
}
