// Package users provides user data models and types.
package users

import (
	"time"
)

// Role represents the user's role in the system.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleOwner Role = "owner"
	RoleAgent Role = "agent"
)

// IsValid checks if the role is a valid role type.
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleOwner, RoleAgent:
		return true
	}
	return false
}

// UserStatus represents the approval status of a user.
type UserStatus string

const (
	StatusPending  UserStatus = "pending"
	StatusApproved UserStatus = "approved"
	StatusRejected UserStatus = "rejected"
)

// IsValid checks if the status is a valid status type.
func (s UserStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected:
		return true
	}
	return false
}

// User represents a user in the system.
type User struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // USER#<phone>
	SK string `dynamodbav:"SK"` // PROFILE

	// GSI1 for querying by role
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // ROLE#<role>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // USER#<phone>

	// User fields
	Phone             string     `dynamodbav:"phone" json:"phone"`
	Name              string     `dynamodbav:"name" json:"name"`
	Email             string     `dynamodbav:"email,omitempty" json:"email,omitempty"`
	Role              Role       `dynamodbav:"role" json:"role"`
	Status            UserStatus `dynamodbav:"status" json:"status"`
	PasswordHash      string     `dynamodbav:"passwordHash,omitempty" json:"-"`
	ManagedProperties []string   `dynamodbav:"managedProperties,omitempty" json:"managedProperties,omitempty"`

	// Metadata
	CreatedAt  time.Time  `dynamodbav:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time  `dynamodbav:"updatedAt" json:"updatedAt"`
	ApprovedBy string     `dynamodbav:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedAt *time.Time `dynamodbav:"approvedAt,omitempty" json:"approvedAt,omitempty"`

	// Entity type for single-table design
	EntityType string `dynamodbav:"entityType" json:"-"`
}

// NewUser creates a new user with initialized fields.
func NewUser(phone, name string, role Role) *User {
	now := time.Now()
	return &User{
		PK:                "USER#" + phone,
		SK:                "PROFILE",
		GSI1PK:            "ROLE#" + string(role),
		GSI1SK:            "USER#" + phone,
		Phone:             phone,
		Name:              name,
		Role:              role,
		Status:            StatusApproved, // Users are auto-approved on OTP verification
		CreatedAt:         now,
		UpdatedAt:         now,
		EntityType:        "USER",
		ManagedProperties: []string{},
	}
}

// HasPassword checks if the user has a password set.
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// IsApproved checks if the user is approved.
func (u *User) IsApproved() bool {
	return u.Status == StatusApproved
}

// CanLogin checks if the user can log in.
func (u *User) CanLogin() bool {
	// All users can login after OTP verification
	return true
}

// UserResponse is the API response representation of a user.
type UserResponse struct {
	Phone             string     `json:"phone"`
	Name              string     `json:"name"`
	Email             string     `json:"email,omitempty"`
	Role              Role       `json:"role"`
	Status            UserStatus `json:"status"`
	ManagedProperties []string   `json:"managedProperties,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// ToResponse converts a User to a UserResponse.
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		Phone:             u.Phone,
		Name:              u.Name,
		Email:             u.Email,
		Role:              u.Role,
		Status:            u.Status,
		ManagedProperties: u.ManagedProperties,
		CreatedAt:         u.CreatedAt,
		UpdatedAt:         u.UpdatedAt,
	}
}
