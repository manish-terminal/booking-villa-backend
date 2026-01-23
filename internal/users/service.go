package users

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/db"
)

// Service provides user-related operations.
type Service struct {
	db *db.Client
}

// NewService creates a new user service.
func NewService(dbClient *db.Client) *Service {
	return &Service{db: dbClient}
}

// CreateUser stores a new user in DynamoDB.
func (s *Service) CreateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	// Try to create only if user doesn't exist
	err := s.db.PutItemWithCondition(ctx, user, "attribute_not_exists(PK)")
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByPhone retrieves a user by phone number.
func (s *Service) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	var user User
	pk := "USER#" + phone
	sk := "PROFILE"

	err := s.db.GetItem(ctx, pk, sk, &user)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user.
func (s *Service) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	return s.db.PutItem(ctx, user)
}

// UpdateUserStatus updates a user's approval status.
func (s *Service) UpdateUserStatus(ctx context.Context, phone string, status UserStatus, approvedBy string) error {
	pk := "USER#" + phone
	sk := "PROFILE"
	now := time.Now().Format(time.RFC3339)

	params := db.UpdateParams{
		UpdateExpression: "SET #status = :status, updatedAt = :updatedAt, approvedBy = :approvedBy, approvedAt = :approvedAt",
		ExpressionValues: map[string]interface{}{
			":status":     string(status),
			":updatedAt":  now,
			":approvedBy": approvedBy,
			":approvedAt": now,
		},
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}

// UpdatePassword sets or updates a user's password.
func (s *Service) UpdatePassword(ctx context.Context, phone string, hashedPassword string) error {
	pk := "USER#" + phone
	sk := "PROFILE"
	now := time.Now().Format(time.RFC3339)

	params := db.UpdateParams{
		UpdateExpression: "SET passwordHash = :passwordHash, updatedAt = :updatedAt",
		ExpressionValues: map[string]interface{}{
			":passwordHash": hashedPassword,
			":updatedAt":    now,
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}

// ListUsersByRole retrieves all users with a specific role.
func (s *Service) ListUsersByRole(ctx context.Context, role Role) ([]*User, error) {
	params := db.QueryParams{
		IndexName:    "GSI1",
		KeyCondition: "GSI1PK = :gsi1pk",
		ExpressionValues: map[string]interface{}{
			":gsi1pk": "ROLE#" + string(role),
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by role: %w", err)
	}

	users := make([]*User, 0, len(items))
	for _, item := range items {
		var user User
		if err := attributevalue.UnmarshalMap(item, &user); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user: %w", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// ListPendingUsers retrieves all users pending approval.
func (s *Service) ListPendingUsers(ctx context.Context) ([]*User, error) {
	// Since we don't have a GSI on status, we'll need to scan
	// For production, consider adding a GSI on status
	// For now, we'll query by role and filter
	var allPending []*User

	for _, role := range []Role{RoleOwner, RoleAgent} {
		users, err := s.ListUsersByRole(ctx, role)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if u.Status == StatusPending {
				allPending = append(allPending, u)
			}
		}
	}

	return allPending, nil
}

// GetOrCreateUser retrieves a user or creates a new one if not found.
// Used during OTP verification to auto-create users.
func (s *Service) GetOrCreateUser(ctx context.Context, phone, name string, role Role) (*User, bool, error) {
	existing, err := s.GetUserByPhone(ctx, phone)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		return existing, false, nil
	}

	// Create new user
	newUser := NewUser(phone, name, role)
	if err := s.CreateUser(ctx, newUser); err != nil {
		return nil, false, err
	}

	return newUser, true, nil
}

// LinkProperty associates a property with a user (agent).
func (s *Service) LinkProperty(ctx context.Context, phone, propertyID string) error {
	pk := "USER#" + phone
	sk := "PROFILE"
	now := time.Now().Format(time.RFC3339)

	// Use list_append to add propertyID to ManagedProperties if it doesn't already exist
	// In a complete implementation, we'd check if it already exists in the list.
	// For now, we'll use a simple UpdateItem that sets/appends.

	params := db.UpdateParams{
		UpdateExpression: "SET managedProperties = list_append(if_not_exists(managedProperties, :empty_list), :property_id), updatedAt = :updatedAt",
		ExpressionValues: map[string]interface{}{
			":property_id": []string{propertyID},
			":empty_list":  []string{},
			":updatedAt":   now,
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}

// IsAuthorizedForProperty checks if a user has permission to manage a property.
func (s *Service) IsAuthorizedForProperty(ctx context.Context, phone string, propertyID string) (bool, error) {
	user, err := s.GetUserByPhone(ctx, phone)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	// Admins are always authorized
	if user.Role == RoleAdmin {
		return true, nil
	}

	// Check if user owns the property or is linked to it
	for _, mp := range user.ManagedProperties {
		if mp == propertyID {
			return true, nil
		}
	}

	return false, nil
}

// ListAgentsForOwner retrieves agents whose ManagedProperties overlap with owner's properties.
// If ownerPropertyIDs is nil or empty, returns all agents (for Admin use).
func (s *Service) ListAgentsForOwner(ctx context.Context, ownerPropertyIDs []string) ([]*User, error) {
	allAgents, err := s.ListUsersByRole(ctx, RoleAgent)
	if err != nil {
		return nil, err
	}

	// If no property filter, return all agents (admin view)
	if len(ownerPropertyIDs) == 0 {
		return allAgents, nil
	}

	// Create a set of owner properties for fast lookup
	ownerPropSet := make(map[string]bool)
	for _, pid := range ownerPropertyIDs {
		ownerPropSet[pid] = true
	}

	// Filter agents by property overlap
	var filtered []*User
	for _, agent := range allAgents {
		for _, agentProp := range agent.ManagedProperties {
			if ownerPropSet[agentProp] {
				filtered = append(filtered, agent)
				break
			}
		}
	}

	return filtered, nil
}

// SetAgentActive activates or deactivates an agent.
func (s *Service) SetAgentActive(ctx context.Context, agentPhone string, isActive bool, setBy string) error {
	status := StatusRejected
	if isActive {
		status = StatusApproved
	}
	return s.UpdateUserStatus(ctx, agentPhone, status, setBy)
}
