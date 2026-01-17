package middleware

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/booking-villa-backend/internal/users"
)

// RBACMiddleware provides role-based access control.
type RBACMiddleware struct {
	authMiddleware *AuthMiddleware
}

// NewRBACMiddleware creates a new RBAC middleware.
func NewRBACMiddleware() *RBACMiddleware {
	return &RBACMiddleware{
		authMiddleware: NewAuthMiddleware(),
	}
}

// RequireRoles returns middleware that requires the user to have one of the specified roles.
func (m *RBACMiddleware) RequireRoles(roles ...users.Role) func(Handler) Handler {
	return func(handler Handler) Handler {
		return func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
			// First, authenticate the user
			authenticated := m.authMiddleware.Authenticate(func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
				// Get claims from context
				claims, ok := GetClaimsFromContext(ctx)
				if !ok {
					return errorResponse(http.StatusUnauthorized, "Unauthorized"), nil
				}

				// Check if user's role is in the allowed roles
				userRole := users.Role(claims.Role)
				allowed := false
				for _, role := range roles {
					if userRole == role {
						allowed = true
						break
					}
				}

				if !allowed {
					return errorResponse(http.StatusForbidden, "Insufficient permissions"), nil
				}

				return handler(ctx, request)
			})

			return authenticated(ctx, request)
		}
	}
}

// RequireAdmin returns middleware that requires admin role.
func (m *RBACMiddleware) RequireAdmin() func(Handler) Handler {
	return m.RequireRoles(users.RoleAdmin)
}

// RequireAdminOrOwner returns middleware that requires admin or owner role.
func (m *RBACMiddleware) RequireAdminOrOwner() func(Handler) Handler {
	return m.RequireRoles(users.RoleAdmin, users.RoleOwner)
}

// RequireAny returns middleware that requires any valid authenticated user.
func (m *RBACMiddleware) RequireAny() func(Handler) Handler {
	return m.RequireRoles(users.RoleAdmin, users.RoleOwner, users.RoleAgent)
}

// CheckOwnership is a helper to verify resource ownership.
// It's typically used within handlers after RBAC check.
func CheckOwnership(ctx context.Context, resourceOwnerID string) bool {
	claims, ok := GetClaimsFromContext(ctx)
	if !ok {
		return false
	}

	// Admins can access any resource
	if claims.Role == string(users.RoleAdmin) {
		return true
	}

	// Check if the user owns the resource
	return claims.Phone == resourceOwnerID || claims.UserID == resourceOwnerID
}

// RoleHierarchy defines the role hierarchy for permission checks.
var RoleHierarchy = map[users.Role]int{
	users.RoleAdmin: 3,
	users.RoleOwner: 2,
	users.RoleAgent: 1,
}

// HasHigherOrEqualRole checks if role1 has higher or equal privilege than role2.
func HasHigherOrEqualRole(role1, role2 users.Role) bool {
	return RoleHierarchy[role1] >= RoleHierarchy[role2]
}

// CanManageUser checks if the requesting user can manage another user.
func CanManageUser(ctx context.Context, targetRole users.Role) bool {
	claims, ok := GetClaimsFromContext(ctx)
	if !ok {
		return false
	}

	requesterRole := users.Role(claims.Role)

	// Only allow management of users with lower privilege
	return RoleHierarchy[requesterRole] > RoleHierarchy[targetRole]
}
