// Package middleware provides authentication and authorization middleware.
package middleware

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/utils"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// UserClaimsKey is the context key for user claims.
	UserClaimsKey ContextKey = "userClaims"
)

// AuthMiddleware wraps a handler function to require JWT authentication.
type AuthMiddleware struct{}

// NewAuthMiddleware creates a new auth middleware instance.
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// Handler type for Lambda handlers.
type Handler func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

// Authenticate wraps a handler to require valid JWT token.
func (m *AuthMiddleware) Authenticate(handler Handler) Handler {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		// Extract Authorization header (case-insensitive)
		authHeader := request.Headers["Authorization"]
		if authHeader == "" {
			authHeader = request.Headers["authorization"]
		}

		if authHeader == "" {
			return errorResponse(http.StatusUnauthorized, "Authorization header required"), nil
		}

		// Extract token from "Bearer <token>"
		tokenString, err := utils.ExtractTokenFromHeader(authHeader)
		if err != nil {
			return errorResponse(http.StatusUnauthorized, "Invalid authorization header format"), nil
		}

		// Validate the token
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			return errorResponse(http.StatusUnauthorized, "Invalid or expired token"), nil
		}

		// Add claims to context
		ctx = context.WithValue(ctx, UserClaimsKey, claims)

		// Also add claims to request headers for easy access
		request.Headers["X-User-Phone"] = claims.Phone
		request.Headers["X-User-Role"] = claims.Role
		request.Headers["X-User-ID"] = claims.UserID

		return handler(ctx, request)
	}
}

// OptionalAuth wraps a handler to extract JWT if present but not require it.
func (m *AuthMiddleware) OptionalAuth(handler Handler) Handler {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		authHeader := request.Headers["Authorization"]
		if authHeader == "" {
			authHeader = request.Headers["authorization"]
		}

		if authHeader != "" {
			tokenString, err := utils.ExtractTokenFromHeader(authHeader)
			if err == nil {
				claims, err := utils.ValidateToken(tokenString)
				if err == nil {
					ctx = context.WithValue(ctx, UserClaimsKey, claims)
					request.Headers["X-User-Phone"] = claims.Phone
					request.Headers["X-User-Role"] = claims.Role
					request.Headers["X-User-ID"] = claims.UserID
				}
			}
		}

		return handler(ctx, request)
	}
}

// GetClaimsFromContext retrieves the user claims from the context.
func GetClaimsFromContext(ctx context.Context) (*utils.TokenClaims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*utils.TokenClaims)
	return claims, ok
}

// GetUserPhoneFromRequest extracts the user phone from request headers.
func GetUserPhoneFromRequest(request events.APIGatewayProxyRequest) string {
	return request.Headers["X-User-Phone"]
}

// GetUserRoleFromRequest extracts the user role from request headers.
func GetUserRoleFromRequest(request events.APIGatewayProxyRequest) string {
	return request.Headers["X-User-Role"]
}

// errorResponse creates an error response for the middleware.
func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,Authorization",
		},
		Body: `{"error":"` + message + `"}`,
	}
}
