// Package notifications provides in-app notification services.
package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/booking-villa-backend/internal/db"
)

// Service provides notification-related operations.
type Service struct {
	db *db.Client
}

// NewService creates a new notification service.
func NewService(dbClient *db.Client) *Service {
	return &Service{db: dbClient}
}

// CreateNotification stores a new notification in DynamoDB.
func (s *Service) CreateNotification(ctx context.Context, notification *Notification) error {
	if err := s.db.PutItem(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// GetNotificationsByUser retrieves notifications for a user.
// Results are ordered by newest first (descending).
func (s *Service) GetNotificationsByUser(ctx context.Context, userPhone string, limit int32, unreadOnly bool) ([]*Notification, error) {
	params := db.QueryParams{
		KeyCondition: "PK = :pk AND begins_with(SK, :skPrefix)",
		ExpressionValues: map[string]interface{}{
			":pk":       "USER#" + userPhone,
			":skPrefix": "NOTIFICATION#",
		},
		Limit: limit,
	}

	if unreadOnly {
		params.FilterExpression = "isRead = :isRead"
		params.ExpressionValues[":isRead"] = false
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}

	notifications := make([]*Notification, 0, len(items))
	for _, item := range items {
		var notif Notification
		if err := attributevalue.UnmarshalMap(item, &notif); err != nil {
			return nil, fmt.Errorf("failed to unmarshal notification: %w", err)
		}
		notifications = append(notifications, &notif)
	}

	return notifications, nil
}

// MarkAsRead marks a notification as read.
func (s *Service) MarkAsRead(ctx context.Context, notificationID, userPhone string) error {
	// First, find the notification by ID using GSI1
	params := db.QueryParams{
		IndexName:    "GSI1",
		KeyCondition: "GSI1PK = :gsi1pk AND GSI1SK = :gsi1sk",
		ExpressionValues: map[string]interface{}{
			":gsi1pk": "NOTIFICATION#" + notificationID,
			":gsi1sk": "USER#" + userPhone,
		},
		Limit: 1,
	}

	items, err := s.db.Query(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to find notification: %w", err)
	}

	if len(items) == 0 {
		return fmt.Errorf("notification not found")
	}

	var notif Notification
	if err := attributevalue.UnmarshalMap(items[0], &notif); err != nil {
		return fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	// Update the notification
	now := time.Now().Format(time.RFC3339)
	updateParams := db.UpdateParams{
		UpdateExpression: "SET isRead = :isRead, updatedAt = :updatedAt",
		ExpressionValues: map[string]interface{}{
			":isRead":    true,
			":updatedAt": now,
		},
	}

	return s.db.UpdateItem(ctx, notif.PK, notif.SK, updateParams)
}

// MarkAllAsRead marks all notifications as read for a user.
func (s *Service) MarkAllAsRead(ctx context.Context, userPhone string) (int, error) {
	// Get all unread notifications
	notifications, err := s.GetNotificationsByUser(ctx, userPhone, 100, true)
	if err != nil {
		return 0, err
	}

	now := time.Now().Format(time.RFC3339)
	count := 0
	for _, notif := range notifications {
		updateParams := db.UpdateParams{
			UpdateExpression: "SET isRead = :isRead, updatedAt = :updatedAt",
			ExpressionValues: map[string]interface{}{
				":isRead":    true,
				":updatedAt": now,
			},
		}

		if err := s.db.UpdateItem(ctx, notif.PK, notif.SK, updateParams); err != nil {
			// Log error but continue with other notifications
			continue
		}
		count++
	}

	return count, nil
}

// GetUnreadCount returns the count of unread notifications for a user.
func (s *Service) GetUnreadCount(ctx context.Context, userPhone string) (int, error) {
	notifications, err := s.GetNotificationsByUser(ctx, userPhone, 100, true)
	if err != nil {
		return 0, err
	}
	return len(notifications), nil
}

// CreateBookingNotification creates a notification for a booking event.
func (s *Service) CreateBookingNotification(ctx context.Context, userPhone string, notifType NotificationType, bookingID, propertyID, propertyName, guestName string) error {
	title, message := generateBookingMessage(notifType, propertyName, guestName)

	notification := NewNotification(userPhone, notifType, title, message)
	notification.BookingID = bookingID
	notification.PropertyID = propertyID

	return s.CreateNotification(ctx, notification)
}

// generateBookingMessage generates title and message for booking notifications.
func generateBookingMessage(notifType NotificationType, propertyName, guestName string) (string, string) {
	switch notifType {
	case TypeBookingCreated:
		return "New Booking", fmt.Sprintf("New booking created for %s by %s", propertyName, guestName)
	case TypeBookingSettled:
		return "Booking Settled", fmt.Sprintf("Booking for %s has been fully settled", propertyName)
	case TypeBookingPartial:
		return "Payment Received", fmt.Sprintf("Partial payment received for %s", propertyName)
	case TypeBookingCancelled:
		return "Booking Cancelled", fmt.Sprintf("Booking for %s has been cancelled", propertyName)
	default:
		return "Booking Update", fmt.Sprintf("Booking for %s has been updated", propertyName)
	}
}
