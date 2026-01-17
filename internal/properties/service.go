// Package properties provides property management services.
package properties

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/db"
	"github.com/google/uuid"
)

// Property represents a property (hotel/villa) in the system.
type Property struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // PROPERTY#<id>
	SK string `dynamodbav:"SK"` // METADATA

	// GSI1 for querying by owner
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // OWNER#<ownerId>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // PROPERTY#<id>

	// Property fields
	ID            string   `dynamodbav:"id" json:"id"`
	Name          string   `dynamodbav:"name" json:"name"`
	Description   string   `dynamodbav:"description,omitempty" json:"description,omitempty"`
	Address       string   `dynamodbav:"address" json:"address"`
	City          string   `dynamodbav:"city" json:"city"`
	State         string   `dynamodbav:"state,omitempty" json:"state,omitempty"`
	Country       string   `dynamodbav:"country" json:"country"`
	OwnerID       string   `dynamodbav:"ownerId" json:"ownerId"`
	OwnerName     string   `dynamodbav:"ownerName,omitempty" json:"ownerName,omitempty"`
	PricePerNight float64  `dynamodbav:"pricePerNight" json:"pricePerNight"`
	Currency      string   `dynamodbav:"currency" json:"currency"`
	MaxGuests     int      `dynamodbav:"maxGuests" json:"maxGuests"`
	Bedrooms      int      `dynamodbav:"bedrooms" json:"bedrooms"`
	Bathrooms     int      `dynamodbav:"bathrooms" json:"bathrooms"`
	Amenities     []string `dynamodbav:"amenities,omitempty" json:"amenities,omitempty"`
	Images        []string `dynamodbav:"images,omitempty" json:"images,omitempty"`
	IsActive      bool     `dynamodbav:"isActive" json:"isActive"`

	// Metadata
	CreatedAt  time.Time `dynamodbav:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time `dynamodbav:"updatedAt" json:"updatedAt"`
	EntityType string    `dynamodbav:"entityType" json:"-"`
}

// InviteCode represents a property-specific invite code for agents.
type InviteCode struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // INVITE#<code>
	SK string `dynamodbav:"SK"` // PROPERTY#<propertyId>

	// GSI1 for querying by property
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // PROPERTY#<propertyId>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // INVITE#<code>

	// Code fields
	Code         string    `dynamodbav:"code" json:"code"`
	PropertyID   string    `dynamodbav:"propertyId" json:"propertyId"`
	PropertyName string    `dynamodbav:"propertyName,omitempty" json:"propertyName,omitempty"`
	CreatedBy    string    `dynamodbav:"createdBy" json:"createdBy"`
	CreatedAt    time.Time `dynamodbav:"createdAt" json:"createdAt"`
	ExpiresAt    time.Time `dynamodbav:"expiresAt" json:"expiresAt"`
	TTL          int64     `dynamodbav:"TTL"` // Auto-delete after expiry
	MaxUses      int       `dynamodbav:"maxUses,omitempty" json:"maxUses,omitempty"`
	UsedCount    int       `dynamodbav:"usedCount" json:"usedCount"`
	IsActive     bool      `dynamodbav:"isActive" json:"isActive"`
	EntityType   string    `dynamodbav:"entityType" json:"-"`
}

// Service provides property-related operations.
type Service struct {
	db *db.Client
}

// NewService creates a new property service.
func NewService(dbClient *db.Client) *Service {
	return &Service{db: dbClient}
}

// CreateProperty creates a new property.
func (s *Service) CreateProperty(ctx context.Context, property *Property) error {
	if property.ID == "" {
		property.ID = uuid.New().String()
	}

	now := time.Now()
	property.PK = "PROPERTY#" + property.ID
	property.SK = "METADATA"
	property.GSI1PK = "OWNER#" + property.OwnerID
	property.GSI1SK = "PROPERTY#" + property.ID
	property.CreatedAt = now
	property.UpdatedAt = now
	property.IsActive = true
	property.EntityType = "PROPERTY"

	if property.Currency == "" {
		property.Currency = "INR"
	}

	return s.db.PutItem(ctx, property)
}

// GetProperty retrieves a property by ID.
func (s *Service) GetProperty(ctx context.Context, id string) (*Property, error) {
	var property Property
	pk := "PROPERTY#" + id
	sk := "METADATA"

	err := s.db.GetItem(ctx, pk, sk, &property)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get property: %w", err)
	}

	return &property, nil
}

// UpdateProperty updates an existing property.
func (s *Service) UpdateProperty(ctx context.Context, property *Property) error {
	property.UpdatedAt = time.Now()
	property.PK = "PROPERTY#" + property.ID
	property.SK = "METADATA"
	return s.db.PutItem(ctx, property)
}

// ListPropertiesByOwner retrieves all properties owned by a user.
func (s *Service) ListPropertiesByOwner(ctx context.Context, ownerID string) ([]*Property, error) {
	params := db.QueryParams{
		IndexName:    "GSI1",
		KeyCondition: "GSI1PK = :gsi1pk",
		ExpressionValues: map[string]string{
			":gsi1pk": "OWNER#" + ownerID,
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list properties: %w", err)
	}

	properties := make([]*Property, 0, len(items))
	for _, item := range items {
		var property Property
		if err := attributevalue.UnmarshalMap(item, &property); err != nil {
			return nil, fmt.Errorf("failed to unmarshal property: %w", err)
		}
		properties = append(properties, &property)
	}

	return properties, nil
}

// GenerateInviteCode creates a new invite code for a property.
func (s *Service) GenerateInviteCode(ctx context.Context, propertyID, createdBy string, expiresAt time.Time, maxUses int) (*InviteCode, error) {
	// Get property to validate it exists and get name
	property, err := s.GetProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	if property == nil {
		return nil, fmt.Errorf("property not found")
	}

	// Generate a random code (8 characters)
	codeBytes := make([]byte, 4)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}
	code := hex.EncodeToString(codeBytes)

	now := time.Now()
	inviteCode := &InviteCode{
		PK:           "INVITE#" + code,
		SK:           "PROPERTY#" + propertyID,
		GSI1PK:       "PROPERTY#" + propertyID,
		GSI1SK:       "INVITE#" + code,
		Code:         code,
		PropertyID:   propertyID,
		PropertyName: property.Name,
		CreatedBy:    createdBy,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		TTL:          expiresAt.Unix(),
		MaxUses:      maxUses,
		UsedCount:    0,
		IsActive:     true,
		EntityType:   "INVITE_CODE",
	}

	if err := s.db.PutItem(ctx, inviteCode); err != nil {
		return nil, fmt.Errorf("failed to create invite code: %w", err)
	}

	return inviteCode, nil
}

// ValidateInviteCode validates an invite code and returns it if valid.
func (s *Service) ValidateInviteCode(ctx context.Context, code string) (*InviteCode, error) {
	// Query by PK to find the invite code
	params := db.QueryParams{
		KeyCondition: "PK = :pk",
		ExpressionValues: map[string]string{
			":pk": "INVITE#" + code,
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query invite code: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("invite code not found")
	}

	var inviteCode InviteCode
	if err := attributevalue.UnmarshalMap(items[0], &inviteCode); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invite code: %w", err)
	}

	// Validate code
	if !inviteCode.IsActive {
		return nil, fmt.Errorf("invite code is no longer active")
	}

	if time.Now().After(inviteCode.ExpiresAt) {
		return nil, fmt.Errorf("invite code has expired")
	}

	if inviteCode.MaxUses > 0 && inviteCode.UsedCount >= inviteCode.MaxUses {
		return nil, fmt.Errorf("invite code has reached maximum uses")
	}

	return &inviteCode, nil
}

// UseInviteCode increments the usage count of an invite code.
func (s *Service) UseInviteCode(ctx context.Context, code, propertyID string) error {
	pk := "INVITE#" + code
	sk := "PROPERTY#" + propertyID

	params := db.UpdateParams{
		UpdateExpression: "SET usedCount = usedCount + :inc",
		ExpressionValues: map[string]string{
			":inc": "1",
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}

// ListInviteCodesByProperty retrieves all invite codes for a property.
func (s *Service) ListInviteCodesByProperty(ctx context.Context, propertyID string) ([]*InviteCode, error) {
	params := db.QueryParams{
		IndexName:    "GSI1",
		KeyCondition: "GSI1PK = :gsi1pk",
		ExpressionValues: map[string]string{
			":gsi1pk": "PROPERTY#" + propertyID,
		},
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list invite codes: %w", err)
	}

	codes := make([]*InviteCode, 0, len(items))
	for _, item := range items {
		var code InviteCode
		if err := attributevalue.UnmarshalMap(item, &code); err != nil {
			continue // Skip invalid entries
		}
		if code.EntityType == "INVITE_CODE" {
			codes = append(codes, &code)
		}
	}

	return codes, nil
}

// DeactivateInviteCode deactivates an invite code.
func (s *Service) DeactivateInviteCode(ctx context.Context, code, propertyID string) error {
	pk := "INVITE#" + code
	sk := "PROPERTY#" + propertyID

	params := db.UpdateParams{
		UpdateExpression: "SET isActive = :isActive",
		ExpressionValues: map[string]string{
			":isActive": "false",
		},
	}

	return s.db.UpdateItem(ctx, pk, sk, params)
}
