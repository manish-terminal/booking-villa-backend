// Package notifications provides in-app notification services.
package notifications

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification.
type NotificationType string

const (
	TypeBookingCreated      NotificationType = "booking_created"
	TypeBookingSettled      NotificationType = "booking_settled"
	TypeBookingPartial      NotificationType = "booking_partial"
	TypeBookingCancelled    NotificationType = "booking_cancelled"
	TypeBookingStatusChange NotificationType = "booking_status_changed"
)

// Notification represents an in-app notification.
type Notification struct {
	// DynamoDB keys
	PK string `dynamodbav:"PK"` // USER#<phone>
	SK string `dynamodbav:"SK"` // NOTIFICATION#<timestamp>#<id>

	// GSI1 for querying by notification ID
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"` // NOTIFICATION#<id>
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"` // USER#<phone>

	// Notification fields
	ID         string           `dynamodbav:"id" json:"id"`
	UserPhone  string           `dynamodbav:"userPhone" json:"userPhone"`
	Type       NotificationType `dynamodbav:"type" json:"type"`
	Title      string           `dynamodbav:"title" json:"title"`
	Message    string           `dynamodbav:"message" json:"message"`
	BookingID  string           `dynamodbav:"bookingId,omitempty" json:"bookingId,omitempty"`
	PropertyID string           `dynamodbav:"propertyId,omitempty" json:"propertyId,omitempty"`
	IsRead     bool             `dynamodbav:"isRead" json:"isRead"`

	// Metadata
	CreatedAt  time.Time `dynamodbav:"createdAt" json:"createdAt"`
	EntityType string    `dynamodbav:"entityType" json:"-"`
}

// NewNotification creates a new notification with initialized fields.
func NewNotification(userPhone string, notifType NotificationType, title, message string) *Notification {
	now := time.Now()
	id := uuid.New().String()
	// Use reverse timestamp for descending order (newest first)
	reverseTS := 9999999999999 - now.UnixMilli()

	return &Notification{
		PK:         "USER#" + userPhone,
		SK:         fmt.Sprintf("NOTIFICATION#%d#%s", reverseTS, id),
		GSI1PK:     "NOTIFICATION#" + id,
		GSI1SK:     "USER#" + userPhone,
		ID:         id,
		UserPhone:  userPhone,
		Type:       notifType,
		Title:      title,
		Message:    message,
		IsRead:     false,
		CreatedAt:  now,
		EntityType: "NOTIFICATION",
	}
}

// NotificationResponse is the API response representation.
type NotificationResponse struct {
	ID         string           `json:"id"`
	Type       NotificationType `json:"type"`
	Title      string           `json:"title"`
	Message    string           `json:"message"`
	BookingID  string           `json:"bookingId,omitempty"`
	PropertyID string           `json:"propertyId,omitempty"`
	IsRead     bool             `json:"isRead"`
	CreatedAt  time.Time        `json:"createdAt"`
}

// ToResponse converts a Notification to a NotificationResponse.
func (n *Notification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID:         n.ID,
		Type:       n.Type,
		Title:      n.Title,
		Message:    n.Message,
		BookingID:  n.BookingID,
		PropertyID: n.PropertyID,
		IsRead:     n.IsRead,
		CreatedAt:  n.CreatedAt,
	}
}
