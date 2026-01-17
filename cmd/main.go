// Package main provides the Lambda entry point for the booking platform backend.
package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/booking-villa-backend/internal/analytics"
	"github.com/booking-villa-backend/internal/auth"
	"github.com/booking-villa-backend/internal/bookings"
	"github.com/booking-villa-backend/internal/db"
	"github.com/booking-villa-backend/internal/middleware"
	"github.com/booking-villa-backend/internal/payments"
	"github.com/booking-villa-backend/internal/properties"
	"github.com/booking-villa-backend/internal/users"
)

// Global handlers (initialized once per Lambda cold start)
var (
	dbClient         *db.Client
	authHandler      *auth.Handler
	propertyHandler  *properties.Handler
	bookingHandler   *bookings.Handler
	paymentHandler   *payments.Handler
	analyticsHandler *analytics.Handler
	authMiddleware   *middleware.AuthMiddleware
	rbacMiddleware   *middleware.RBACMiddleware
	userService      *users.Service
)

func init() {
	ctx := context.Background()

	var err error
	dbClient, err = db.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	// Initialize handlers
	authHandler = auth.NewHandler(dbClient)
	propertyHandler = properties.NewHandler(dbClient)
	bookingHandler = bookings.NewHandler(dbClient)
	paymentHandler = payments.NewHandler(dbClient)
	analyticsHandler = analytics.NewHandler(dbClient)
	userService = users.NewService(dbClient)

	// Initialize middleware
	authMiddleware = middleware.NewAuthMiddleware()
	rbacMiddleware = middleware.NewRBACMiddleware()
}

// Handler is the main Lambda handler that routes requests to appropriate handlers.
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Log request for debugging
	log.Printf("Request: %s %s", request.HTTPMethod, request.Path)

	// Handle CORS preflight
	if request.HTTPMethod == "OPTIONS" {
		return corsResponse(), nil
	}

	// Route the request
	return routeRequest(ctx, request)
}

// corsResponse returns a CORS preflight response.
func corsResponse() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type,Authorization,X-Amz-Date,X-Api-Key,X-Amz-Security-Token",
		},
	}
}

// routeRequest routes the incoming request to the appropriate handler.
func routeRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := request.Path
	method := request.HTTPMethod

	// Normalize path (remove trailing slash)
	path = strings.TrimSuffix(path, "/")

	// Auth routes (public)
	if strings.HasPrefix(path, "/auth") {
		return routeAuth(ctx, request, path, method)
	}

	// User routes
	if strings.HasPrefix(path, "/users") {
		return routeUsers(ctx, request, path, method)
	}

	// Property routes
	if strings.HasPrefix(path, "/properties") {
		return routeProperties(ctx, request, path, method)
	}

	// Invite code validation (public with optional auth)
	if path == "/invite-codes/validate" && method == "POST" {
		return propertyHandler.HandleValidateInviteCode(ctx, request)
	}

	// Booking routes
	if strings.HasPrefix(path, "/bookings") {
		return routeBookings(ctx, request, path, method)
	}

	// Analytics routes
	if strings.HasPrefix(path, "/analytics") {
		return routeAnalytics(ctx, request, path, method)
	}

	// Health check
	if path == "/health" || path == "/" {
		return apiResponse(200, map[string]string{
			"status":  "healthy",
			"service": "booking-villa-backend",
		}), nil
	}

	return errorResponse(404, "Not found"), nil
}

// routeAuth handles authentication routes.
func routeAuth(ctx context.Context, request events.APIGatewayProxyRequest, path, method string) (events.APIGatewayProxyResponse, error) {
	switch {
	case path == "/auth/send-otp" && method == "POST":
		return authHandler.HandleSendOTP(ctx, request)

	case path == "/auth/verify-otp" && method == "POST":
		return authHandler.HandleVerifyOTP(ctx, request)

	case path == "/auth/login" && method == "POST":
		return authHandler.HandleLogin(ctx, request)

	case path == "/auth/refresh" && method == "POST":
		return authHandler.HandleRefreshToken(ctx, request)

	default:
		return errorResponse(404, "Auth endpoint not found"), nil
	}
}

// routeUsers handles user management routes.
func routeUsers(ctx context.Context, request events.APIGatewayProxyRequest, path, method string) (events.APIGatewayProxyResponse, error) {
	// Password setting requires auth
	if path == "/users/password" && method == "POST" {
		return authMiddleware.Authenticate(authHandler.HandleSetPassword)(ctx, request)
	}

	// Get user by phone - requires auth
	if strings.HasPrefix(path, "/users/") && method == "GET" {
		phone := request.PathParameters["phone"]
		if phone == "" {
			// Extract from path
			parts := strings.Split(path, "/")
			if len(parts) >= 3 {
				phone = parts[2]
			}
		}
		return authMiddleware.Authenticate(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
			user, err := userService.GetUserByPhone(ctx, phone)
			if err != nil {
				return errorResponse(500, "Failed to get user"), nil
			}
			if user == nil {
				return errorResponse(404, "User not found"), nil
			}
			return apiResponse(200, user.ToResponse()), nil
		})(ctx, request)
	}

	// List users - admin only
	if path == "/users" && method == "GET" {
		return rbacMiddleware.RequireAdmin()(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
			// Get role filter from query params
			roleFilter := req.QueryStringParameters["role"]

			if roleFilter != "" {
				role := users.Role(roleFilter)
				if !role.IsValid() {
					return errorResponse(400, "Invalid role filter"), nil
				}
				userList, err := userService.ListUsersByRole(ctx, role)
				if err != nil {
					return errorResponse(500, "Failed to list users"), nil
				}
				responses := make([]users.UserResponse, len(userList))
				for i, u := range userList {
					responses[i] = u.ToResponse()
				}
				return apiResponse(200, map[string]interface{}{
					"users": responses,
					"count": len(responses),
				}), nil
			}

			// List pending users by default
			pending, err := userService.ListPendingUsers(ctx)
			if err != nil {
				return errorResponse(500, "Failed to list pending users"), nil
			}
			responses := make([]users.UserResponse, len(pending))
			for i, u := range pending {
				responses[i] = u.ToResponse()
			}
			return apiResponse(200, map[string]interface{}{
				"users": responses,
				"count": len(responses),
			}), nil
		})(ctx, request)
	}

	// Update user status - admin only
	if strings.HasSuffix(path, "/status") && method == "PATCH" {
		return rbacMiddleware.RequireAdmin()(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
			phone := req.PathParameters["phone"]
			if phone == "" {
				// Extract from path
				parts := strings.Split(path, "/")
				if len(parts) >= 3 {
					phone = parts[2]
				}
			}

			var body struct {
				Status string `json:"status"`
			}
			if err := parseBody(req.Body, &body); err != nil {
				return errorResponse(400, "Invalid request body"), nil
			}

			status := users.UserStatus(body.Status)
			if !status.IsValid() {
				return errorResponse(400, "Invalid status"), nil
			}

			claims, _ := middleware.GetClaimsFromContext(ctx)
			if err := userService.UpdateUserStatus(ctx, phone, status, claims.Phone); err != nil {
				return errorResponse(500, "Failed to update user status"), nil
			}

			return apiResponse(200, map[string]string{
				"message": "User status updated",
				"phone":   phone,
				"status":  string(status),
			}), nil
		})(ctx, request)
	}

	return errorResponse(404, "User endpoint not found"), nil
}

// routeProperties handles property routes.
func routeProperties(ctx context.Context, request events.APIGatewayProxyRequest, path, method string) (events.APIGatewayProxyResponse, error) {
	// Check for invite codes endpoint
	if strings.Contains(path, "/invite-codes") {
		if method == "POST" {
			return rbacMiddleware.RequireAdminOrOwner()(propertyHandler.HandleGenerateInviteCode)(ctx, request)
		}
		if method == "GET" {
			return rbacMiddleware.RequireAdminOrOwner()(propertyHandler.HandleListInviteCodes)(ctx, request)
		}
	}

	// Check for availability endpoint
	if strings.HasSuffix(path, "/availability") && method == "GET" {
		return bookingHandler.HandleCheckAvailability(ctx, request)
	}

	// Check for calendar endpoint
	if strings.HasSuffix(path, "/calendar") && method == "GET" {
		return bookingHandler.HandleGetPropertyCalendar(ctx, request)
	}

	switch {
	case path == "/properties" && method == "POST":
		return rbacMiddleware.RequireAdminOrOwner()(propertyHandler.HandleCreateProperty)(ctx, request)

	case path == "/properties" && method == "GET":
		return authMiddleware.Authenticate(propertyHandler.HandleListProperties)(ctx, request)

	case strings.HasPrefix(path, "/properties/") && method == "GET":
		return propertyHandler.HandleGetProperty(ctx, request)

	case strings.HasPrefix(path, "/properties/") && method == "PATCH":
		return rbacMiddleware.RequireAdminOrOwner()(propertyHandler.HandleUpdateProperty)(ctx, request)

	default:
		return errorResponse(404, "Property endpoint not found"), nil
	}
}

// routeBookings handles booking and payment routes.
func routeBookings(ctx context.Context, request events.APIGatewayProxyRequest, path, method string) (events.APIGatewayProxyResponse, error) {
	// Check for payment endpoints
	if strings.Contains(path, "/payments") {
		if method == "POST" {
			return rbacMiddleware.RequireAny()(paymentHandler.HandleLogPayment)(ctx, request)
		}
		if method == "GET" {
			return authMiddleware.Authenticate(paymentHandler.HandleGetPayments)(ctx, request)
		}
	}

	// Check for payment status endpoint
	if strings.HasSuffix(path, "/payment-status") && method == "GET" {
		return authMiddleware.Authenticate(paymentHandler.HandleGetPaymentStatus)(ctx, request)
	}

	// Check for booking status endpoint
	if strings.HasSuffix(path, "/status") && method == "PATCH" {
		return rbacMiddleware.RequireAdminOrOwner()(bookingHandler.HandleUpdateBookingStatus)(ctx, request)
	}

	switch {
	case path == "/bookings" && method == "POST":
		return rbacMiddleware.RequireAny()(bookingHandler.HandleCreateBooking)(ctx, request)

	case path == "/bookings" && method == "GET":
		return authMiddleware.Authenticate(bookingHandler.HandleListBookings)(ctx, request)

	case strings.HasPrefix(path, "/bookings/") && method == "GET":
		return authMiddleware.Authenticate(bookingHandler.HandleGetBooking)(ctx, request)

	default:
		return errorResponse(404, "Booking endpoint not found"), nil
	}
}

// routeAnalytics handles analytics routes.
func routeAnalytics(ctx context.Context, request events.APIGatewayProxyRequest, path, method string) (events.APIGatewayProxyResponse, error) {
	switch {
	case path == "/analytics/owner" && method == "GET":
		return rbacMiddleware.RequireAdminOrOwner()(analyticsHandler.HandleOwnerAnalytics)(ctx, request)

	case path == "/analytics/agent" && method == "GET":
		return rbacMiddleware.RequireAny()(analyticsHandler.HandleAgentAnalytics)(ctx, request)

	case path == "/analytics/dashboard" && method == "GET":
		return authMiddleware.Authenticate(analyticsHandler.HandleDashboard)(ctx, request)

	default:
		return errorResponse(404, "Analytics endpoint not found"), nil
	}
}

// Helper functions

func apiResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
	return auth.APIResponse(statusCode, body)
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return auth.ErrorResponse(statusCode, message)
}

func parseBody(body string, v interface{}) error {
	return json.Unmarshal([]byte(body), v)
}

func main() {
	lambda.Start(Handler)
}
